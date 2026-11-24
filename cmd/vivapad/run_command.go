package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/runcommand"
)

var runVariableNames = [...]string{
	"FULL_CURRENT_PATH", "CURRENT_DIRECTORY", "FILE_NAME", "NAME_PART", "EXT_PART",
	"CURRENT_WORD", "NPP_DIRECTORY", "NPP_FULL_FILE_PATH", "CURRENT_LINE", "CURRENT_COLUMN", "CURRENT_LINESTR",
}

func (state *model) openRunDialog() {
	state.runDialogOpen, state.runSaveMode, state.runVariablesOpen = true, false, false
	state.runInput, state.runName = text.NewEditor(state.runLast), nil
	state.runDialogError, state.runDialogFocus = "", true
}

func (state *model) cancelRunDialog() {
	if state.runVariablesOpen {
		state.runVariablesOpen = false
		return
	}
	if state.runSaveMode {
		state.runSaveMode, state.runName, state.runDialogError = false, nil, ""
		state.runDialogFocus = true
		return
	}
	state.closeRunDialog()
}

func (state *model) closeRunDialog() {
	if state.runInput != nil {
		state.runLast = state.runInput.String()
	}
	state.runDialogOpen, state.runSaveMode, state.runVariablesOpen = false, false, false
	state.runInput, state.runName = nil, nil
	state.runDialogError = ""
	state.requestEditorCommandFocus()
}

func runCurrentWord(doc *document.Document) string {
	const limit = 1024
	editor := doc.Editor
	value := editor.String()
	selection := editor.Selection()
	if selection.End > selection.Start {
		start := editor.ByteOffset(selection.Start)
		end := editor.ByteOffset(min(selection.End, selection.Start+limit))
		return value[start:end]
	}
	word := func(character rune) bool {
		return unicode.IsLetter(character) || unicode.IsNumber(character) || character == '_'
	}
	start := editor.ByteOffset(editor.Caret())
	character, size := utf8.DecodeRuneInString(value[start:])
	if !word(character) {
		character, size = utf8.DecodeLastRuneInString(value[:start])
		if !word(character) {
			return ""
		}
		start -= size
	}
	end, count := start+size, 1
	for start > 0 {
		character, size := utf8.DecodeLastRuneInString(value[:start])
		if !word(character) {
			break
		}
		start -= size
		count++
	}
	for end < len(value) && count < limit {
		character, size := utf8.DecodeRuneInString(value[end:])
		if !word(character) {
			break
		}
		end += size
		count++
	}
	return value[start:end]
}

func (state *model) runVariables(doc *document.Document) runcommand.Variables {
	line, column := doc.Position()
	value, offset := doc.Editor.String(), doc.Editor.ByteOffset(doc.Editor.Caret())
	start := strings.LastIndexByte(value[:offset], '\n') + 1
	end := len(value)
	if next := strings.IndexByte(value[offset:], '\n'); next >= 0 {
		end = offset + next
	}
	appPath, _ := os.Executable()
	lineText := value[start:end]
	const maxLineBytes = 4096
	if len(lineText) > maxLineBytes {
		cut := maxLineBytes
		for !utf8.RuneStart(lineText[cut]) {
			cut--
		}
		lineText = lineText[:cut]
	}
	return runcommand.Variables{Path: doc.Path, Word: runCurrentWord(doc), Line: line, Column: column, LineText: lineText, AppPath: appPath}
}

func (state *model) launchRunTemplate(template string) error {
	prepared, err := runcommand.Prepare(template, state.runVariables(state.paneDocument(state.activePane)))
	if err != nil {
		return err
	}
	if state.runLauncher == nil {
		return errors.New("Run launcher unavailable")
	}
	if err := state.runLauncher(prepared); err != nil {
		return err
	}
	state.notice = "Launched " + filepath.Base(prepared.Executable)
	return nil
}

func (state *model) applyRunDialog() {
	if !state.runDialogOpen {
		return
	}
	if state.runSaveMode {
		candidate := append(append([]runcommand.Named(nil), state.namedRuns...), runcommand.Named{Name: state.runName.String(), Template: state.runInput.String()})
		if err := runcommand.ValidateNamed(candidate); err != nil {
			state.runDialogError = err.Error()
			return
		}
		if state.runPath == "" {
			state.runDialogError = "Run command storage unavailable"
			return
		}
		if err := runcommand.Save(state.runPath, candidate); err != nil {
			state.runDialogError = err.Error()
			return
		}
		state.namedRuns = candidate
		name := state.runName.String()
		state.closeRunDialog()
		state.notice = "Saved Run command " + name
		return
	}
	if err := state.launchRunTemplate(state.runInput.String()); err != nil {
		state.runDialogError = err.Error()
		return
	}
	state.closeRunDialog()
}

func (state *model) browseRunExecutable() {
	input := state.runInput
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: "Program to Run"})
	if input == nil || !state.runDialogOpen || state.runSaveMode || state.runInput != input {
		return
	}
	state.runDialogFocus = true
	if errors.Is(err, app.ErrCanceled) {
		return
	}
	if err != nil {
		state.runDialogError = err.Error()
		return
	}
	if len(paths) == 0 {
		return
	}
	input.Insert(strconv.Quote(paths[0]))
	state.runDialogError = ""
}
