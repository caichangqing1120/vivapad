package macro

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

type Kind string

const (
	TextStep    Kind = "text"
	CodeStep    Kind = "code"
	LineStep    Kind = "line"
	CommentStep Kind = "comment"

	MaxSteps         = 1024
	MaxReplays       = 9999
	maxBytes         = 1 << 20
	maxRunes         = 4 << 20
	maxPlaybackSteps = 1 << 20
)

type Step struct {
	Kind       Kind                         `json:"kind"`
	TextKind   app.TextEventKind            `json:"text_kind,omitempty"`
	Text       string                       `json:"text,omitempty"`
	Command    app.TextCommand              `json:"command,omitempty"`
	Code       components.CodeEditorCommand `json:"code,omitempty"`
	Line       string                       `json:"line,omitempty"`
	Comment    string                       `json:"comment,omitempty"`
	LinePrefix string                       `json:"line_prefix,omitempty"`
	BlockStart string                       `json:"block_start,omitempty"`
	BlockEnd   string                       `json:"block_end,omitempty"`
}

type diskState struct {
	Version int    `json:"version"`
	Steps   []Step `json:"steps"`
}

func Validate(steps []Step) error {
	if len(steps) > MaxSteps {
		return fmt.Errorf("macro exceeds %d steps", MaxSteps)
	}
	totalText := 0
	for _, step := range steps {
		totalText += len(step.Text)
		if totalText > maxBytes {
			return fmt.Errorf("macro text exceeds 1 MiB")
		}
		switch step.Kind {
		case TextStep:
			if step.Code != 0 || step.Line != "" || hasCommentFields(step) ||
				step.TextKind != app.TextInsert && step.TextKind != app.TextPaste && step.TextKind != app.TextCommandEvent ||
				step.TextKind == app.TextCommandEvent && (step.Command < app.TextDeleteBackward || step.Command > app.TextDeleteToLineEnd || step.Command == app.TextUndo || step.Command == app.TextRedo || step.Text != "") ||
				step.TextKind != app.TextCommandEvent && step.Command != 0 {
				return fmt.Errorf("invalid macro text step")
			}
		case CodeStep:
			if step.TextKind != 0 || step.Text != "" || step.Command != 0 || step.Line != "" || hasCommentFields(step) ||
				step.Code < components.CodeEditorInsertTab || step.Code > components.CodeEditorDeleteLines {
				return fmt.Errorf("invalid macro code step")
			}
		case LineStep:
			if step.TextKind != 0 || step.Text != "" || step.Command != 0 || step.Code != 0 || hasCommentFields(step) ||
				step.Line != "Duplicate Line" && step.Line != "Delete Line" && step.Line != "Indent Lines" && step.Line != "Unindent Lines" {
				return fmt.Errorf("invalid macro line step")
			}
		case CommentStep:
			if step.TextKind != 0 || step.Text != "" || step.Command != 0 || step.Code != 0 || step.Line != "" ||
				step.Comment != "line-toggle" && step.Comment != "line-add" && step.Comment != "line-remove" &&
					step.Comment != "block-add" && step.Comment != "block-remove" ||
				step.LinePrefix == "" && (step.BlockStart == "" || step.BlockEnd == "") ||
				(step.BlockStart == "") != (step.BlockEnd == "") ||
				!validCommentToken(step.LinePrefix) || !validCommentToken(step.BlockStart) || !validCommentToken(step.BlockEnd) {
				return fmt.Errorf("invalid macro comment step")
			}
		default:
			return fmt.Errorf("unsupported macro step %q", step.Kind)
		}
	}
	return nil
}

func hasCommentFields(step Step) bool {
	return step.Comment != "" || step.LinePrefix != "" || step.BlockStart != "" || step.BlockEnd != ""
}

func validCommentToken(value string) bool {
	if len(value) > 32 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func ApplyComment(doc *document.Document, step Step) bool {
	switch step.Comment {
	case "line-toggle", "line-add", "line-remove":
		mode := document.LineCommentToggle
		if step.Comment == "line-add" {
			mode = document.LineCommentAdd
		} else if step.Comment == "line-remove" {
			mode = document.LineCommentRemove
		}
		if step.LinePrefix != "" {
			changed := doc.ChangeLineComments(step.LinePrefix, mode)
			if changed || mode != document.LineCommentRemove || step.BlockStart == "" {
				return changed
			}
			return doc.RemoveBlockComments(step.BlockStart, step.BlockEnd)
		}
		if mode == document.LineCommentRemove {
			return doc.RemoveBlockComments(step.BlockStart, step.BlockEnd)
		}
		return doc.ChangeStreamLineComments(step.BlockStart, step.BlockEnd, mode)
	case "block-add":
		if step.BlockStart != "" {
			return doc.AddBlockComment(step.BlockStart, step.BlockEnd)
		}
		return doc.ChangeLineComments(step.LinePrefix, document.LineCommentAdd)
	case "block-remove":
		if step.BlockStart != "" {
			return doc.RemoveBlockComments(step.BlockStart, step.BlockEnd)
		}
		return doc.ChangeLineComments(step.LinePrefix, document.LineCommentRemove)
	}
	return false
}

func Play(doc *document.Document, steps []Step, repeats int) error {
	return PlayWithExecutor(doc, steps, repeats, nil)
}

func PlayWithExecutor(doc *document.Document, steps []Step, repeats int, execute func(*document.Document, Step)) error {
	return play(doc, steps, repeats, false, execute)
}

func PlayUntilEnd(doc *document.Document, steps []Step) error {
	return PlayUntilEndWithExecutor(doc, steps, nil)
}

func PlayUntilEndWithExecutor(doc *document.Document, steps []Step, execute func(*document.Document, Step)) error {
	return play(doc, steps, MaxReplays, true, execute)
}

func play(doc *document.Document, steps []Step, repeats int, untilEnd bool, execute func(*document.Document, Step)) error {
	if doc == nil || doc.Editor == nil || len(steps) == 0 || repeats < 1 || repeats > MaxReplays || !untilEnd && len(steps)*repeats > maxPlaybackSteps {
		return fmt.Errorf("macro or repeat count is invalid")
	}
	if err := Validate(steps); err != nil {
		return err
	}
	if _, composing := doc.Editor.Composition(); composing {
		return fmt.Errorf("finish input-method composition before playing a macro")
	}
	return doc.Editor.Transact(func(editor *text.Editor) error {
		lineDoc := document.NewDraft("macro", editor)
		return playDraft(lineDoc, steps, repeats, untilEnd, execute)
	})
}

func playDraft(lineDoc *document.Document, steps []Step, repeats int, untilEnd bool, execute func(*document.Document, Step)) error {
	editor := lineDoc.Editor
	initialRevision := editor.TrackEdits()
	for iteration := 0; iteration < repeats; iteration++ {
		if untilEnd && editor.Caret() >= editor.RuneCount() {
			break
		}
		if untilEnd && (iteration+1)*len(steps) > maxPlaybackSteps {
			return fmt.Errorf("macro replay limit reached")
		}
		remaining := editor.RuneCount() - editor.Caret()
		for _, step := range steps {
			if execute != nil {
				execute(lineDoc, step)
			} else {
				switch step.Kind {
				case TextStep:
					app.TextEvent{Kind: step.TextKind, Text: step.Text, Command: step.Command}.Apply(editor)
				case CodeStep:
					step.Code.Apply(editor)
				case LineStep:
					switch step.Line {
					case "Duplicate Line":
						lineDoc.DuplicateLine()
					case "Delete Line":
						lineDoc.DeleteLine()
					case "Indent Lines":
						lineDoc.IndentLines()
					case "Unindent Lines":
						lineDoc.UnindentLines()
					}
				case CommentStep:
					ApplyComment(lineDoc, step)
				}
			}
			if editor.RuneCount() > maxRunes {
				return fmt.Errorf("macro result exceeds 4 Mi runes")
			}
			if editor.TrackEdits()-initialRevision > maxPlaybackSteps {
				return fmt.Errorf("macro primitive edit limit reached")
			}
		}
		if untilEnd {
			if editor.RuneCount()-editor.Caret() >= remaining {
				return fmt.Errorf("macro did not advance toward the end of file")
			}
			if iteration == repeats-1 && editor.Caret() < editor.RuneCount() {
				return fmt.Errorf("macro replay limit reached")
			}
		}
	}
	return nil
}

func Save(path string, steps []Step) error {
	if err := Validate(steps); err != nil {
		return err
	}
	data, err := json.Marshal(diskState{Version: 1, Steps: steps})
	if err != nil {
		return err
	}
	if len(data) > maxBytes {
		return fmt.Errorf("macro exceeds 1 MiB")
	}
	return writePrivate(path, data)
}

func writePrivate(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("macro directory is not a regular directory")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".vivapad-macro-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func Load(path string) ([]Step, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("macro exceeds 1 MiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxBytes))
	decoder.DisallowUnknownFields()
	var stored diskState
	if err := decoder.Decode(&stored); err != nil {
		return nil, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("macro has trailing data")
	}
	if stored.Version != 1 {
		return nil, fmt.Errorf("unsupported macro version %d", stored.Version)
	}
	if err := Validate(stored.Steps); err != nil {
		return nil, err
	}
	return stored.Steps, nil
}

func Clear(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
