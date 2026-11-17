package main

import (
	"fmt"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/macro"
)

func (state *model) macroTextInput(doc *document.Document) *app.TextInput {
	return &app.TextInput{State: app.TextStateOf(doc.Editor), OnEvent: func(event app.TextEvent) (app.TextState, error) {
		if state.isReadOnly(doc) {
			state.cancelDocumentComposition(doc)
			state.notice = "Read-only document"
			return app.TextStateOf(doc.Editor), nil
		}
		if event.CompositionPhase != text.CompositionNone && doc.Editor.TentativeCompositionEnabled() {
			return state.applyPublicComposition(doc, event), nil
		}
		if current := state.wordCompletion; current != nil && current.doc == doc && state.validWordCompletion() &&
			event.Kind == app.TextCommandEvent && (event.Command == app.TextInsertNewline || event.Command == app.TextInsertLineBreak) {
			state.acceptWordCompletion(current.selected)
			return app.TextStateOf(doc.Editor), nil
		}
		committed := ""
		beforeSelection, beforeCaret := doc.Editor.Selection(), doc.Editor.Caret()
		beforeText := doc.Editor.String()
		if event.Kind == app.TextPaste && event.Rectangular {
			if beforeSelection.Start != beforeSelection.End {
				state.notice = "Clear the selection before column paste"
				return app.TextStateOf(doc.Editor), nil
			}
			var pasteError error
			state.editWithBookmarkAnchor(doc, func() {
				_, pasteError = doc.PasteRectangleLines(document.Rectangle{Anchor: beforeCaret, Caret: beforeCaret, TabWidth: state.tabWidth}, event.Text)
			})
			if pasteError != nil {
				state.notice = pasteError.Error()
				return app.TextStateOf(doc.Editor), nil
			}
			state.relocateSelectAnchors(doc, beforeText, doc.Editor.String(), beforeSelection.Start)
			if state.macroRecording {
				state.cancelMacroUnsupported("column paste")
			}
			return app.TextStateOf(doc.Editor), nil
		}
		state.bookmarkLines(doc)
		if event.Kind == app.TextUnmark {
			before := app.TextStateOf(doc.Editor)
			if before.Composing {
				runes := []rune(before.Value)
				if before.Composition.Start >= 0 && before.Composition.End <= len(runes) {
					committed = string(runes[before.Composition.Start:before.Composition.End])
				}
			}
		}
		state.editWithBookmarkAnchor(doc, func() { event.Apply(doc.Editor) })
		if event.Kind == app.TextCommandEvent && (event.Command == app.TextUndo || event.Command == app.TextRedo) {
			state.restoreRectangleAcrossViews(doc, beforeText, doc.Editor.String(), event.Command == app.TextUndo)
			state.restoreMultiSelectionAcrossViews(doc, beforeText, doc.Editor.String(), event.Command == app.TextUndo)
		}
		if state.wordCompletion != nil && state.wordCompletion.doc == doc {
			if event.Kind == app.TextInsert && event.Text != "" {
				state.updateWordCompletionAfterInput(doc)
			} else if event.Kind != app.TextSetSelection || !state.validWordCompletion() {
				state.wordCompletion = nil
			}
		}
		state.relocateSelectAnchors(doc, beforeText, doc.Editor.String(), beforeSelection.Start)
		if state.macroRecording {
			if event.Kind == app.TextSetSelection && (doc.Editor.Selection() != beforeSelection || doc.Editor.Caret() != beforeCaret) {
				state.cancelMacroUnsupported("selection")
			}
			switch event.Kind {
			case app.TextInsert, app.TextPaste:
				if event.Text != "" {
					state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: event.Kind, Text: event.Text})
				}
			case app.TextCommandEvent:
				if event.Command == app.TextUndo || event.Command == app.TextRedo {
					state.cancelMacroUnsupported("undo or redo")
				} else {
					state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: event.Kind, Command: event.Command})
				}
			case app.TextUnmark:
				if committed != "" {
					state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: app.TextInsert, Text: committed})
				}
			}
		}
		return app.TextStateOf(doc.Editor), nil
	}}
}

func (state *model) startMacro() {
	if state.macroRecording {
		return
	}
	if state.searchOpen || state.goToOpen || state.filesDialogOpen || state.replaceConfirmOpen || state.macroSaveOpen || state.macroRunOpen || state.pendingClose >= 0 || state.pendingReload >= 0 {
		state.notice = "Close dialogs before recording a macro"
		return
	}
	state.macroRecording, state.macroSource = true, state.space.Current()
	state.macroSteps, state.macroTextBytes = nil, 0
	state.notice = "Recording macro"
}

func (state *model) stopMacro() {
	if !state.macroRecording {
		state.notice = "No macro is recording"
		return
	}
	state.macroRecording, state.macroSource = false, nil
	if len(state.macroSteps) == 0 {
		state.notice = "No actions recorded; previous macro kept"
		return
	}
	state.macroLast = append([]macro.Step(nil), state.macroSteps...)
	state.macroSteps = nil
	if state.macroPath != "" {
		if err := macro.Save(state.macroPath, state.macroLast); err != nil {
			state.notice = "Macro not saved: " + err.Error()
			return
		}
	}
	state.notice = fmt.Sprintf("Recorded %d macro steps", len(state.macroLast))
}

func (state *model) recordMacro(doc *document.Document, step macro.Step) {
	if !state.macroRecording {
		return
	}
	if doc != state.macroSource {
		state.cancelMacroOnSwitch()
		return
	}
	if len(state.macroSteps) >= macro.MaxSteps || state.macroTextBytes+len(step.Text) > 1<<20 {
		state.macroRecording, state.macroSource, state.macroSteps = false, nil, nil
		state.notice = "Recording limit reached; previous macro kept"
		return
	}
	state.macroSteps = append(state.macroSteps, step)
	state.macroTextBytes += len(step.Text)
	state.notice = ""
}

func (state *model) checkMacroSource() {
	if state.macroRecording && state.space.Current() != state.macroSource {
		state.cancelMacroOnSwitch()
	}
}

func (state *model) cancelMacroOnSwitch() {
	state.macroRecording, state.macroSource, state.macroSteps = false, nil, nil
	state.notice = "Recording canceled after changing document"
}

func (state *model) cancelMacroUnsupported(action string) {
	state.macroRecording, state.macroSource, state.macroSteps = false, nil, nil
	state.notice = "Recording canceled: unsupported " + action
}

func (state *model) cancelMacroForCommand() {
	if state.macroRecording {
		state.cancelMacroUnsupported("command")
	}
}

func macroMenuSupported(command string) bool {
	switch command {
	case "Record Macro", "Stop Recording", "Playback Macro", "Playback 5 Times", "Clear Macro", "Save Current Recorded Macro...", "Run a Macro Multiple Times...", "Saved Macros",
		"Cut", "Copy", "Paste", "Delete", "Select All",
		"Duplicate Line", "Delete Line", "Indent Lines", "Unindent Lines", "Comment/Uncomment",
		"Toggle Single Line Comment", "Single Line Comment", "Single Line Uncomment", "Block Comment", "Block Uncomment":
		return true
	}
	return false
}

func (state *model) playMacro(repeats int) {
	state.playMacroSteps(state.macroLast, repeats, false)
}

func (state *model) playMacroSteps(steps []macro.Step, repeats int, untilEnd bool) bool {
	if state.isReadOnly(state.space.Current()) {
		state.notice = "Read-only document"
		return false
	}
	if state.macroRecording {
		state.notice = "Stop recording before playback"
		return false
	}
	doc := state.space.Current()
	marked := append([]int(nil), state.bookmarkLines(doc)...)
	var shadow *document.Document
	var finalMarks []int
	var execute func(*document.Document, macro.Step)
	if len(marked) != 0 {
		execute = func(preview *document.Document, step macro.Step) {
			if shadow == nil {
				shadow = preview
				state.setBookmarkLines(shadow, marked)
			}
			switch step.Kind {
			case macro.TextStep:
				state.editWithBookmarkAnchor(shadow, func() {
					app.TextEvent{Kind: step.TextKind, Text: step.Text, Command: step.Command}.Apply(shadow.Editor)
				})
			case macro.CodeStep:
				state.applyCodeCommand(shadow, step.Code)
			case macro.CommentStep:
				state.editWithBookmarkAnchor(shadow, func() { macro.ApplyComment(shadow, step) })
			case macro.LineStep:
				switch step.Line {
				case "Duplicate Line":
					state.applyCodeCommand(shadow, components.CodeEditorDuplicateLinesDown)
				case "Delete Line":
					state.deleteLineWithBookmarks(shadow)
				case "Indent Lines":
					state.editWithBookmarkAnchor(shadow, shadow.IndentLines)
				case "Unindent Lines":
					state.editWithBookmarkAnchor(shadow, shadow.UnindentLines)
				}
			}
			finalMarks = append(finalMarks[:0], state.bookmarkLines(shadow)...)
		}
	}
	var err error
	if untilEnd {
		err = macro.PlayUntilEndWithExecutor(doc, steps, execute)
	} else {
		err = macro.PlayWithExecutor(doc, steps, repeats, execute)
	}
	if shadow != nil {
		delete(state.bookmarks, shadow)
	}
	if err != nil {
		state.notice = err.Error()
		return false
	}
	if shadow != nil {
		state.setBookmarkLines(doc, finalMarks)
	}
	if untilEnd {
		state.notice = "Played macro to end of file"
	} else {
		state.notice = fmt.Sprintf("Played macro %d time(s)", repeats)
	}
	return true
}

func (state *model) clearMacro() {
	if state.macroPath != "" {
		if err := macro.Clear(state.macroPath); err != nil {
			state.notice = "Macro not cleared: " + err.Error()
			return
		}
	}
	state.macroRecording, state.macroSource = false, nil
	state.macroSteps, state.macroLast = nil, nil
	state.notice = "Macro cleared"
}
