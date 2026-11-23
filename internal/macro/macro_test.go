package macro

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestPlayUsesCurrentCaretAndOneUndoUnit(t *testing.T) {
	doc := document.New("target.txt")
	doc.Editor.Insert("abc")
	doc.Editor.ClearHistory()
	steps := []Step{
		{Kind: TextStep, TextKind: app.TextInsert, Text: "X"},
		{Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextMoveLeft},
		{Kind: TextStep, TextKind: app.TextInsert, Text: "Y"},
	}
	if err := Play(doc, steps, 1); err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != "abcYX" {
		t.Fatalf("played text = %q", got)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "abc" {
		t.Fatalf("macro was not one undo unit: %q", doc.Editor.String())
	}
}

func TestPlayKeepsCodeEditingOptionsAndCanRepeat(t *testing.T) {
	doc := document.New("source.go")
	doc.Editor.SetCode(text.CodeOptions{TabWidth: 2, LineComment: "//"})
	doc.Editor.Insert("value")
	doc.Editor.MoveTo(0, false)
	steps := []Step{
		{Kind: CodeStep, Code: components.CodeEditorToggleComment},
		{Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextMoveDocumentEnd},
		{Kind: TextStep, TextKind: app.TextInsert, Text: "!"},
	}
	if err := Play(doc, steps, 2); err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != "value!!" {
		t.Fatalf("repeated code macro = %q", got)
	}
}

func TestPlayAcceptsFourDigitRepeatAndRejectsOverflowAtomically(t *testing.T) {
	doc := document.New("repeat.txt")
	doc.Editor.Insert("base")
	doc.Editor.ClearHistory()
	steps := []Step{{Kind: TextStep, TextKind: app.TextInsert, Text: "!"}}
	if err := Play(doc, steps, 101); err != nil || doc.Editor.String() != "base"+strings.Repeat("!", 101) {
		t.Fatalf("101 repeats = %q, %v", doc.Editor.String(), err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "base" {
		t.Fatal("repeated macro was not one undo unit")
	}
	if err := Play(doc, steps, 10000); err == nil || doc.Editor.String() != "base" {
		t.Fatalf("invalid count changed document = %q, %v", doc.Editor.String(), err)
	}
}

func TestPlayUntilEndRequiresProgressAndCommitsOnce(t *testing.T) {
	doc := document.New("until-eof.txt")
	doc.Editor.Insert("abc")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	steps := []Step{
		{Kind: TextStep, TextKind: app.TextInsert, Text: "!"},
		{Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextMoveRight},
	}
	if err := PlayUntilEnd(doc, steps); err != nil || doc.Editor.String() != "!a!b!c" || doc.Editor.Caret() != doc.Editor.RuneCount() {
		t.Fatalf("EOF playback = %q, caret=%d, %v", doc.Editor.String(), doc.Editor.Caret(), err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "abc" {
		t.Fatal("EOF playback was not one undo unit")
	}
	doc.Editor.MoveTo(0, false)
	before := doc.Editor.Selection()
	if err := PlayUntilEnd(doc, steps[:1]); err == nil || doc.Editor.String() != "abc" || doc.Editor.Selection() != before {
		t.Fatalf("non-progress macro changed document = %q, selection=%+v, %v", doc.Editor.String(), doc.Editor.Selection(), err)
	}
}

func TestPlaybackUpperBoundsDoNotCommitPartialEOFResult(t *testing.T) {
	doc := document.New("upper.txt")
	doc.Editor.Insert("a")
	doc.Editor.ClearHistory()
	if err := Play(doc, []Step{{Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextMoveLeft}}, MaxReplays); err != nil {
		t.Fatalf("maximum four-digit repeat rejected: %v", err)
	}
	if doc.Editor.String() != "a" || doc.Editor.CanUndo() {
		t.Fatal("cursor-only repeats edited the document")
	}
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: doc.Editor.RuneCount()}, strings.Repeat("x", MaxReplays+1))
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	if err := PlayUntilEnd(doc, []Step{{Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextMoveRight}}); err == nil ||
		doc.Editor.Caret() != 0 || doc.Editor.String() != strings.Repeat("x", MaxReplays+1) || doc.Editor.CanUndo() {
		t.Fatalf("EOF replay limit changed source: caret=%d err=%v", doc.Editor.Caret(), err)
	}
}

func TestLegacyCodeToggleMacroKeepsItsOriginalInterpretation(t *testing.T) {
	doc := document.New("legacy.go")
	doc.Editor.SetCode(text.CodeOptions{LineComment: "//"})
	doc.Editor.Insert("// one\ntwo")
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	if err := Play(doc, []Step{{Kind: CodeStep, Code: components.CodeEditorToggleComment}}, 1); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "// // one\n// two" {
		t.Fatalf("legacy CodeStep behavior changed: %q", doc.Editor.String())
	}
}

func TestCommentMacroReplaysMixedLineToggleAndPersistsDelimiters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "macro.json")
	steps := []Step{{Kind: CommentStep, Comment: "line-toggle", LinePrefix: "//"}}
	if err := Save(path, steps); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || !reflect.DeepEqual(loaded, steps) {
		t.Fatalf("comment macro round-trip = %+v, %v", loaded, err)
	}
	doc := document.New("mixed.go")
	doc.Editor.Insert("// one\ntwo")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	if err := Play(doc, loaded, 1); err != nil || doc.Editor.String() != "one\n// two" {
		t.Fatalf("comment macro playback = %q, %v", doc.Editor.String(), err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "// one\ntwo" {
		t.Fatal("comment macro should be one undo unit")
	}
	stream := []Step{{Kind: CommentStep, Comment: "line-toggle", BlockStart: "<!--", BlockEnd: "-->"}}
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: doc.Editor.RuneCount()}, "<!-- one -->\ntwo")
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	if err := Play(doc, stream, 1); err != nil || doc.Editor.String() != "one\n<!-- two -->" {
		t.Fatalf("stream-only macro playback = %q, %v", doc.Editor.String(), err)
	}
}

func TestCommentMacroRejectsMissingOrUnsafeDelimiters(t *testing.T) {
	for _, step := range []Step{
		{Kind: CommentStep, Comment: "line-toggle"},
		{Kind: CommentStep, Comment: "line-toggle", BlockStart: "/*"},
		{Kind: CommentStep, Comment: "line-toggle", LinePrefix: "//\n"},
		{Kind: CommentStep, Comment: "line-toggle", LinePrefix: "  "},
		{Kind: CommentStep, Comment: "unknown", LinePrefix: "//"},
		{Kind: CommentStep, Comment: "line-toggle", LinePrefix: "//", Text: "unvalidated"},
	} {
		if err := Validate([]Step{step}); err == nil {
			t.Fatalf("unsafe comment macro accepted: %+v", step)
		}
	}
}

func TestPlayWithExecutorRunsValidatedStepsInSandboxAndCommitsOneEdit(t *testing.T) {
	doc := document.New("sandbox.txt")
	doc.Editor.Insert("base")
	doc.Editor.ClearHistory()
	steps := []Step{{Kind: TextStep, TextKind: app.TextInsert, Text: "!"}}
	var before []string
	execute := func(preview *document.Document, step Step) {
		before = append(before, preview.Editor.String())
		app.TextEvent{Kind: step.TextKind, Text: step.Text}.Apply(preview.Editor)
	}
	if err := PlayWithExecutor(doc, steps, 2, execute); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, []string{"base", "base!"}) || doc.Editor.String() != "base!!" {
		t.Fatalf("sandbox steps = %v, document = %q", before, doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "base" {
		t.Fatalf("sandbox replay did not commit one undo unit: %q", doc.Editor.String())
	}
	called := false
	invalid := []Step{{Kind: LineStep, Line: "unknown"}}
	if err := PlayWithExecutor(doc, invalid, 1, func(*document.Document, Step) { called = true }); err == nil || called || doc.Editor.String() != "base" {
		t.Fatalf("invalid macro ran executor: called=%v, text=%q, err=%v", called, doc.Editor.String(), err)
	}
}

func TestInvalidOrOversizedPlaybackLeavesDocumentUntouched(t *testing.T) {
	doc := document.New("safe.txt")
	doc.Editor.Insert("keep")
	for _, input := range []struct {
		steps   []Step
		repeats int
	}{
		{[]Step{{Kind: TextStep, TextKind: app.TextInsert, Text: "x"}}, 0},
		{[]Step{{Kind: "unknown"}}, 1},
		{[]Step{{Kind: TextStep, TextKind: app.TextInsert, Text: "x"}}, 10000},
		{[]Step{{Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextUndo}}, 1},
	} {
		if err := Play(doc, input.steps, input.repeats); err == nil || doc.Editor.String() != "keep" {
			t.Fatalf("unsafe playback changed text to %q: %v", doc.Editor.String(), err)
		}
	}
}

func TestMultiSelectionMacroStepValidation(t *testing.T) {
	valid := []Step{
		{Kind: SelectionStep, Selection: SelectAllMatches, SelectFlags: SelectMatchCase | SelectWholeWord},
		{Kind: SelectionStep, Selection: UndoLatestSelection},
		{Kind: MultiTextStep, TextKind: app.TextInsert, Text: "x"},
		{Kind: MultiTextStep, TextKind: app.TextCommandEvent, Command: app.TextDeleteForward},
	}
	if err := Validate(valid); err != nil {
		t.Fatal(err)
	}
	for _, step := range []Step{
		{Kind: SelectionStep, Selection: SelectAllMatches, SelectFlags: 4},
		{Kind: SelectionStep, Selection: SkipCurrentMatch, SelectFlags: SelectWholeWord},
		{Kind: SelectionStep, Selection: SelectNextMatch, Text: "unexpected"},
		{Kind: TextStep, TextKind: app.TextInsert, Selection: SelectAllMatches, Text: "x"},
		{Kind: MultiTextStep, TextKind: app.TextInsert},
		{Kind: MultiTextStep, TextKind: app.TextCommandEvent, Command: app.TextSelectAll},
	} {
		if err := Validate([]Step{step}); err == nil {
			t.Fatalf("invalid step accepted: %+v", step)
		}
	}
}

func TestPrivateMacroRoundTripAndInvalidStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "macro.json")
	want := []Step{{Kind: TextStep, TextKind: app.TextInsert, Text: "中文"}, {Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextMoveLeft}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("saved macro = %+v, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || !privateMode(info.Mode()) {
		t.Fatalf("macro file mode = %+v, %v", info, err)
	}
	info, err = os.Stat(filepath.Dir(path))
	if err != nil || !privateDirMode(info.Mode()) {
		t.Fatalf("macro directory mode = %+v, %v", info, err)
	}
	if err := os.WriteFile(path, []byte(`{"version":2,"steps":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("future macro format accepted")
	}
}

// privateMode reports whether mode carries the private-file metadata this
// platform supports: POSIX 0600 on macOS and Linux, or a writable regular file
// on Windows, where Go reports synthetic permission bits and VivaPad keeps the
// privacy of its stores in the profile directory ACL (a separate gate).
func privateMode(mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return mode.IsRegular() && mode.Perm()&0200 != 0
	}
	return mode.Perm() == 0600
}

// privateDirMode reports whether mode carries a private directory's metadata:
// POSIX 0700 on macOS and Linux, or a writable directory on Windows.
func privateDirMode(mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return mode.IsDir() && mode.Perm()&0200 != 0
	}
	return mode.Perm() == 0700
}
