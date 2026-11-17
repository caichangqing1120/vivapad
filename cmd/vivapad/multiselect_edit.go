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
	characters := []rune(before)
	boundaries := multiGraphemeBoundaries(before)
	positions := make([]int, len(characters)+1)
	for index, character := range characters {
		positions[index+1] = positions[index] + utf8.RuneLen(character)
	}
	type indexedEdit struct {
		index  int
		target text.Range
		value  string
	}
	edits := make([]indexedEdit, 0, len(selected.ranges))
	for index, selection := range selected.ranges {
		if selection.Start < 0 || selection.End > len(characters) || selection.End < selection.Start {
			return false, fmt.Errorf("invalid multi-selection")
		}
		target := selection
		replacement := value
		if kind != multiInsert {
			target = multiDeleteTarget(selection, kind, boundaries)
			replacement = ""
		}
		edits = append(edits, indexedEdit{index: index, target: target, value: replacement})
	}
	slices.SortFunc(edits, func(left, right indexedEdit) int { return left.target.Start - right.target.Start })
	changes := make([]text.Replacement, 0, len(edits))
	byteEdits := make([]document.ReplacementEdit, 0, len(edits))
	carets := make([]text.Range, len(edits))
	shift := 0
	for index, edit := range edits {
		if index > 0 && edit.target.Start < edits[index-1].target.End ||
			index > 0 && edit.target.Start == edits[index-1].target.Start {
			return false, fmt.Errorf("multi-selection deletion ranges overlap")
		}
		changes = append(changes, text.Replacement{Range: edit.target, Text: edit.value})
		if edit.target.Start != edit.target.End || edit.value != "" {
			byteEdits = append(byteEdits, document.ReplacementEdit{
				Start: positions[edit.target.Start], End: positions[edit.target.End], Text: edit.value,
			})
		}
		caret := edit.target.Start + shift + utf8.RuneCountInString(edit.value)
		carets[edit.index] = text.Range{Start: caret, End: caret}
		shift += utf8.RuneCountInString(edit.value) - (edit.target.End - edit.target.Start)
	}
	if len(byteEdits) == 0 {
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

func (state *model) multiSelectionText(pane int) string {
	doc := state.paneDocument(pane)
	state.syncMultiSelection(pane, doc)
	selected := state.multiSelections[pane]
	if !selected.active {
		return ""
	}
	characters := []rune(selected.value)
	var output strings.Builder
	for _, selection := range selected.ranges {
		if selection.Start == selection.End {
			continue
		}
		if output.Len() != 0 {
			output.WriteByte('\n')
		}
		output.WriteString(string(characters[selection.Start:selection.End]))
	}
	return output.String()
}

func (state *model) copyMultiSelection(doc *document.Document, cut bool) {
	value := state.multiSelectionText(state.activePane)
	if value == "" {
		return
	}
	if err := app.SetClipboardText(value); err != nil {
		state.notice = "Copy failed: " + err.Error()
		return
	}
	if cut {
		if _, err := state.editMultiSelection(state.activePane, "", multiInsert); err != nil {
			state.notice = "Cut failed: " + err.Error()
		}
	}
}

func (state *model) multiSelectionTextInput(pane int) *app.TextInput {
	doc := state.paneDocument(pane)
	copyText := state.multiSelectionText(pane)
	return &app.TextInput{State: app.TextStateOf(doc.Editor), CopyOverride: true, CopyText: func() string { return copyText }, OnEvent: func(event app.TextEvent) (app.TextState, error) {
		if state.isReadOnly(doc) {
			state.notice = "Read-only document"
			return app.TextStateOf(doc.Editor), nil
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
		cutting := event.Kind == app.TextPaste && value == "" && copyText != ""
		if cutting {
			value = ""
		} else if event.Kind == app.TextCommandEvent {
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
		} else if event.Kind != app.TextInsert && event.Kind != app.TextPaste || event.Rectangular {
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
