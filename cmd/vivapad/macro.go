package main

import (
	"fmt"
	"slices"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/macro"
	"vivapad/internal/workspace"
)

func (state *model) macroTextInput(doc *document.Document) *app.TextInput {
	return &app.TextInput{State: app.TextStateOf(doc.Editor), OnEvent: func(event app.TextEvent) (app.TextState, error) {
		if event.CompositionPhase != text.CompositionNone {
			next, _ := state.applyMacroComposition(doc, event)
			return next, nil
		}
		if state.isReadOnly(doc) {
			state.cancelDocumentComposition(doc)
			state.notice = "Read-only document"
			return app.TextStateOf(doc.Editor), nil
		}
		if state.provisional.doc == doc {
			state.cancelDocumentComposition(doc)
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
	pane := state.activePane
	state.syncMultiSelection(pane, state.space.Current())
	if state.multiSelections[pane].active {
		state.notice = "Clear the multi-selection before recording a macro"
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
		"Cut", "Copy", "Paste", pasteSpecialCommand, pasteHTMLCommand, pasteRTFCommand, "Delete", "Select All",
		"Multi-select All", "Multi-select Next", "Undo Latest Multi-Select", "Skip Current Multi-Select",
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
	pane := state.activePane
	multiselect := slices.ContainsFunc(steps, func(step macro.Step) bool {
		return step.Kind == macro.SelectionStep || step.Kind == macro.MultiTextStep
	})
	previous := state.multiSelections[pane]
	before := doc.Editor.String()
	if previous.doc != doc || previous.value != before || len(previous.ranges) == 0 || previous.ranges[len(previous.ranges)-1] != doc.Editor.Selection() {
		previous = multiSelectionState{}
	}
	marked := append([]int(nil), state.bookmarkLines(doc)...)
	var shadow *document.Document
	var finalMarks []int
	runner := state
	var execute func(*document.Document, macro.Step) error
	if len(marked) != 0 || multiselect {
		execute = func(preview *document.Document, step macro.Step) error {
			if shadow == nil {
				shadow = preview
				if multiselect {
					runner = &model{
						space:              &workspace.Workspace{Documents: []*document.Document{shadow}},
						bookmarks:          make(map[*document.Document]*bookmarkState),
						multiSelectOptions: state.multiSelectOptions,
					}
				}
				if len(marked) != 0 {
					runner.setBookmarkLines(shadow, marked)
				}
			}
			switch step.Kind {
			case macro.SelectionStep:
				switch step.Selection {
				case macro.SelectAllMatches, macro.SelectNextMatch:
					runner.multiSelect(document.SearchOptions{
						MatchCase: step.SelectFlags&macro.SelectMatchCase != 0,
						WholeWord: step.SelectFlags&macro.SelectWholeWord != 0,
					}, step.Selection == macro.SelectAllMatches)
				case macro.SkipCurrentMatch:
					runner.skipMultiSelect()
				case macro.UndoLatestSelection:
					runner.undoMultiSelect()
				}
			case macro.MultiTextStep:
				runner.syncMultiSelection(0, shadow)
				if !runner.multiSelections[0].active {
					return fmt.Errorf("multi-selection macro has no active ranges")
				}
				if err := runner.replayMultiSelectionText(step); err != nil {
					return err
				}
			case macro.TextStep:
				if multiselect {
					runner.multiSelections[0] = multiSelectionState{}
				}
				runner.editWithBookmarkAnchor(shadow, func() {
					app.TextEvent{Kind: step.TextKind, Text: step.Text, Command: step.Command}.Apply(shadow.Editor)
				})
			case macro.CodeStep:
				runner.applyCodeCommand(shadow, step.Code)
			case macro.CommentStep:
				runner.editWithBookmarkAnchor(shadow, func() { macro.ApplyComment(shadow, step) })
			case macro.LineStep:
				switch step.Line {
				case "Duplicate Line":
					runner.applyCodeCommand(shadow, components.CodeEditorDuplicateLinesDown)
				case "Delete Line":
					runner.deleteLineWithBookmarks(shadow)
				case "Indent Lines":
					runner.editWithBookmarkAnchor(shadow, shadow.IndentLines)
				case "Unindent Lines":
					runner.editWithBookmarkAnchor(shadow, shadow.UnindentLines)
				}
			}
			finalMarks = append(finalMarks[:0], runner.bookmarkLines(shadow)...)
			return nil
		}
	}
	err := macro.PlayChecked(doc, steps, repeats, untilEnd, execute)
	if shadow != nil {
		delete(runner.bookmarks, shadow)
	}
	if err != nil {
		state.notice = err.Error()
		return false
	}
	if shadow != nil && len(marked) != 0 {
		state.setBookmarkLines(doc, finalMarks)
	}
	if multiselect && shadow != nil {
		runner.syncMultiSelection(0, shadow)
		after := doc.Editor.String()
		result := runner.multiSelections[0]
		if result.active {
			result.doc, result.value = doc, after
		}
		state.multiSelections[pane] = result
		state.multiSelectOptions = runner.multiSelectOptions
		state.recordMultiSelectionTransition(pane, doc, before, after, previous)
	}
	if untilEnd {
		state.notice = "Played macro to end of file"
	} else {
		state.notice = fmt.Sprintf("Played macro %d time(s)", repeats)
	}
	// Playback edits the real document source; consume its history once here so
	// the lifecycle path is not left silently unread.
	if _, historyErr := doc.ChangeHistory(); historyErr != nil {
		state.notice += "; " + historyErr.Error()
	}
	return true
}

func (state *model) replayMultiSelectionText(step macro.Step) error {
	kind := multiInsert
	if step.TextKind == app.TextPaste {
		kind = multiPaste
	} else if step.TextKind == app.TextCommandEvent {
		switch step.Command {
		case app.TextDeleteBackward:
			kind = multiDeleteBackward
		case app.TextDeleteForward:
			kind = multiDeleteForward
		}
	}
	_, err := state.editMultiSelection(0, step.Text, kind)
	return err
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
