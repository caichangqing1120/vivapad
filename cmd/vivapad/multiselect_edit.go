package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

type multiEditKind uint8

const (
	multiInsert multiEditKind = iota
	multiDeleteBackward
	multiDeleteForward
)

func multiGraphemeBoundaries(value string) []int {
	boundaries := []int{0}
	runes := 0
	for graphemes := uniseg.NewGraphemes(value); graphemes.Next(); {
		start, end := graphemes.Positions()
		runes += utf8.RuneCountInString(value[start:end])
		boundaries = append(boundaries, runes)
	}
	return boundaries
}

func multiDeleteTarget(selection text.Range, kind multiEditKind, boundaries []int) text.Range {
	if selection.Start < selection.End {
		return selection
	}
	caret := selection.Start
	index, found := slices.BinarySearch(boundaries, caret)
	if !found && index > 0 {
		return text.Range{Start: boundaries[index-1], End: boundaries[index]}
	}
	if kind == multiDeleteBackward && index > 0 {
		return text.Range{Start: boundaries[index-1], End: caret}
	}
	if kind == multiDeleteForward && index+1 < len(boundaries) {
		return text.Range{Start: caret, End: boundaries[index+1]}
	}
	return selection
}

func (state *model) editMultiSelection(pane int, value string, kind multiEditKind) (bool, error) {
	doc := state.paneDocument(pane)
	state.syncMultiSelection(pane, doc)
	selected := state.multiSelections[pane]
	if !selected.active {
		return false, fmt.Errorf("multi-selection changed")
	}
	if state.isReadOnly(doc) {
		return false, fmt.Errorf("read-only document")
	}
	before := doc.Editor.String()
	runeCount := doc.Editor.RuneCount()
	if kind != multiInsert {
		value = ""
	}
	inserted := utf8.RuneCountInString(value)
	var boundaries []int
	type indexedEdit struct {
		index  int
		target text.Range
	}
	edits := make([]indexedEdit, 0, len(selected.ranges))
	for index, selection := range selected.ranges {
		if selection.Start < 0 || selection.End > runeCount || selection.End < selection.Start {
			return false, fmt.Errorf("invalid multi-selection")
		}
		target := selection
		if kind != multiInsert && selection.Start == selection.End {
			if boundaries == nil {
				boundaries = multiGraphemeBoundaries(before)
			}
			target = multiDeleteTarget(selection, kind, boundaries)
		}
		edits = append(edits, indexedEdit{index: index, target: target})
	}
	slices.SortFunc(edits, func(left, right indexedEdit) int { return left.target.Start - right.target.Start })
	changes := make([]text.Replacement, 0, len(edits))
	carets := make([]text.Range, len(edits))
	hasEdits := false
	shift := 0
	for index, edit := range edits {
		if index > 0 && edit.target.Start < edits[index-1].target.End ||
			index > 0 && edit.target.Start == edits[index-1].target.Start {
			return false, fmt.Errorf("multi-selection deletion ranges overlap")
		}
		changes = append(changes, text.Replacement{Range: edit.target, Text: value})
		hasEdits = hasEdits || edit.target.Start != edit.target.End || inserted != 0
		caret := edit.target.Start + shift + inserted
		carets[edit.index] = text.Range{Start: caret, End: caret}
		shift += inserted - (edit.target.End - edit.target.Start)
	}
	if !hasEdits {
		return false, nil
	}
	state.bookmarkLines(doc)
	if err := doc.Editor.ReplaceRanges(changes); err != nil {
		return false, err
	}
	state.bookmarkLines(doc)
	after := doc.Editor.String()
	if before == after {
		return false, nil
	}
	state.multiSelections[pane] = multiSelectionState{doc: doc, value: after, ranges: carets, active: true}
	doc.Editor.MoveTo(carets[len(carets)-1].Start, false)
	state.recordMultiSelectionTransition(pane, doc, before, after, selected)
	state.relocateSelectAnchors(doc, before, after, -1)
	if state.macroRecording {
		state.cancelMacroUnsupported("multi-selection edit")
	}
	return true, nil
}

func (selected multiSelectionState) clipboardText() string {
	if !selected.active {
		return ""
	}
	editor := selected.doc.Editor
	var output strings.Builder
	for _, selection := range selected.ranges {
		if selection.Start == selection.End {
			continue
		}
		if output.Len() != 0 {
			output.WriteByte('\n')
		}
		output.WriteString(selected.value[editor.ByteOffset(selection.Start):editor.ByteOffset(selection.End)])
	}
	return output.String()
}

func (state *model) copyMultiSelection(doc *document.Document, cut bool) {
	pane := state.activePane
	state.syncMultiSelection(pane, doc)
	value := state.multiSelections[pane].clipboardText()
	if value == "" {
		return
	}
	if err := app.SetClipboardText(value); err != nil {
		state.notice = "Copy failed: " + err.Error()
		return
	}
	if cut {
		if _, err := state.editMultiSelection(pane, "", multiInsert); err != nil {
			state.notice = "Cut failed: " + err.Error()
		}
	}
}

func (state *model) multiSelectionTextInput(pane int) *app.TextInput {
	doc := state.paneDocument(pane)
	state.syncMultiSelection(pane, doc)
	inputState := app.TextStateOf(doc.Editor)
	clipboardSelection := func() multiSelectionState {
		// Native clipboard callbacks may outlive the displayed text input.
		if state.paneDocument(pane) != doc || doc.Editor.String() != inputState.Value {
			return multiSelectionState{}
		}
		state.syncMultiSelection(pane, doc)
		return state.multiSelections[pane]
	}
	return &app.TextInput{State: inputState, CopyOverride: true, CopyText: func() string { return clipboardSelection().clipboardText() }, OnEvent: func(event app.TextEvent) (app.TextState, error) {
		if state.isReadOnly(doc) {
			state.cancelDocumentComposition(doc)
			state.notice = "Read-only document"
			return app.TextStateOf(doc.Editor), nil
		}
		if event.CompositionPhase != text.CompositionNone {
			next, handled := state.applyMacroComposition(doc, event)
			if handled {
				state.multiSelections[pane] = multiSelectionState{}
			}
			return next, nil
		}
		if state.provisional.doc == doc {
			state.cancelDocumentComposition(doc)
		}
		if event.Kind == app.TextSetSelection {
			if event.Selection == (app.TextRange{Start: doc.Editor.Caret(), End: doc.Editor.Caret()}) ||
				event.Selection == (app.TextRange{Start: doc.Editor.Selection().Start, End: doc.Editor.Selection().End}) {
				return app.TextStateOf(doc.Editor), nil
			}
		}
		if event.Kind == app.TextPasteNoText {
			return app.TextStateOf(doc.Editor), nil
		}
		kind := multiInsert
		value := event.Text
		cutting := false
		if event.Kind == app.TextPaste && value == "" {
			cutting = slices.ContainsFunc(clipboardSelection().ranges, func(selection text.Range) bool {
				return selection.Start != selection.End
			})
		}
		if event.Kind == app.TextCommandEvent {
			switch event.Command {
			case app.TextDeleteBackward:
				kind = multiDeleteBackward
			case app.TextDeleteForward:
				kind = multiDeleteForward
			case app.TextUndo, app.TextRedo:
				state.bookmarkLines(doc)
				before := doc.Editor.String()
				if event.Command == app.TextUndo && doc.Editor.Undo() || event.Command == app.TextRedo && doc.Editor.Redo() {
					state.restoreMultiSelectionAcrossViews(doc, before, doc.Editor.String(), event.Command == app.TextUndo)
				}
				state.bookmarkLines(doc)
				return app.TextStateOf(doc.Editor), nil
			default:
				state.multiSelections[pane] = multiSelectionState{}
				return state.macroTextInput(doc).OnEvent(event)
			}
		} else if !cutting && (event.Kind != app.TextInsert && event.Kind != app.TextPaste || event.Rectangular) {
			state.multiSelections[pane] = multiSelectionState{}
			return state.macroTextInput(doc).OnEvent(event)
		}
		if kind == multiInsert && value == "" && !cutting {
			return app.TextStateOf(doc.Editor), nil
		}
		if _, err := state.editMultiSelection(pane, value, kind); err != nil {
			state.notice = "Multi-selection edit failed: " + err.Error()
		}
		return app.TextStateOf(doc.Editor), nil
	}}
}
