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
	"vivapad/internal/macro"
)

type multiEditKind uint8

const (
	multiInsert multiEditKind = iota
	multiPaste
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
	deleting := kind == multiDeleteBackward || kind == multiDeleteForward
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
		if deleting && selection.Start == selection.End {
			if boundaries == nil {
				boundaries = multiGraphemeBoundaries(before)
			}
			target = multiDeleteTarget(selection, kind, boundaries)
		}
		edits = append(edits, indexedEdit{index: index, target: target})
	}
	slices.SortFunc(edits, func(left, right indexedEdit) int { return left.target.Start - right.target.Start })
	parts := []string{value}
	if kind == multiPaste {
		parts = document.MultiCursorPasteParts(value, len(edits))
	} else if deleting {
		parts[0] = ""
	}
	changes := make([]text.Replacement, 0, len(edits))
	carets := make([]text.Range, len(edits))
	hasEdits := false
	shift := 0
	for first := 0; first < len(edits); {
		target := edits[first].target
		next := first + 1
		for next < len(edits) && (edits[next].target.Start < target.End || edits[next].target.Start == target.Start) {
			target.End = max(target.End, edits[next].target.End)
			next++
		}
		replacement := parts[0]
		if len(parts) > 1 {
			replacement = parts[first]
		}
		inserted := utf8.RuneCountInString(replacement)
		changes = append(changes, text.Replacement{Range: target, Text: replacement})
		hasEdits = hasEdits || target.Start != target.End || inserted != 0
		caret := target.Start + shift + inserted
		for _, edit := range edits[first:next] {
			carets[edit.index] = text.Range{Start: caret, End: caret}
		}
		shift += inserted - (target.End - target.Start)
		first = next
	}
	if !hasEdits {
		return false, nil
	}
	carets = uniqueMultiCarets(carets)
	main := carets[len(carets)-1]
	state.bookmarkLines(doc)
	if err := doc.Editor.ReplaceRangesWith(changes, text.ReplaceRangesOptions{AfterSelection: &main}); err != nil {
		return false, err
	}
	state.bookmarkLines(doc)
	after := doc.Editor.String()
	state.multiSelections[pane] = multiSelectionState{doc: doc, value: after, ranges: carets, active: true}
	doc.Editor.SetSelection(main)
	state.recordMultiSelectionTransition(pane, doc, before, after, selected)
	state.relocateSelectAnchors(doc, before, after, -1)
	if state.macroRecording {
		step := macro.Step{Kind: macro.MultiTextStep, TextKind: app.TextCommandEvent}
		switch kind {
		case multiInsert:
			if value == "" {
				state.cancelMacroUnsupported("multi-selection cut")
				return true, nil
			}
			step.TextKind, step.Text = app.TextInsert, value
		case multiPaste:
			step.TextKind, step.Text = app.TextPaste, value
		case multiDeleteBackward:
			step.Command = app.TextDeleteBackward
		case multiDeleteForward:
			step.Command = app.TextDeleteForward
		}
		state.recordMacro(doc, step)
	}
	return before != after, nil
}

func (state *model) pasteMultiSelection(pane int, value string) {
	if value != "" {
		if _, err := state.editMultiSelection(pane, value, multiPaste); err != nil {
			state.notice = "Multi-selection paste failed: " + err.Error()
		}
	}
}

func (selected multiSelectionState) hasClipboardText() bool {
	return selected.active && slices.ContainsFunc(selected.ranges, func(selection text.Range) bool {
		return selection.Start != selection.End
	})
}

func (selected multiSelectionState) rectangularClipboard() bool {
	return len(selected.ranges) > 1
}

func (selected multiSelectionState) clipboardText() string {
	if !selected.hasClipboardText() {
		return ""
	}
	doc := selected.doc
	separator := "\n"
	switch doc.LineEnding() {
	case document.CRLF:
		separator = "\r\n"
	case document.CR:
		separator = "\r"
	}
	var output strings.Builder
	for _, selection := range selected.ranges {
		start, end := doc.Editor.ByteOffset(selection.Start), doc.Editor.ByteOffset(selection.End)
		span := selected.value[start:end]
		if strings.Contains(span, "\n") {
			if doc.MixedLineEndings() {
				line := strings.Count(selected.value[:start], "\n")
				for {
					part, rest, found := strings.Cut(span, "\n")
					output.WriteString(part)
					if !found {
						break
					}
					output.WriteString(doc.LineEndingAt(line))
					span = rest
					line++
				}
			} else {
				output.WriteString(strings.ReplaceAll(span, "\n", separator))
			}
		} else {
			output.WriteString(span)
		}
		if selected.rectangularClipboard() {
			output.WriteString(separator)
		}
	}
	return output.String()
}

func (state *model) copyMultiSelection(doc *document.Document, cut bool) {
	pane := state.activePane
	state.syncMultiSelection(pane, doc)
	selected := state.multiSelections[pane]
	value := selected.clipboardText()
	if value == "" {
		return
	}
	writeClipboard := app.SetClipboardText
	if selected.rectangularClipboard() {
		writeClipboard = app.SetRectangularClipboardText
	}
	if err := writeClipboard(value); err != nil {
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
	return &app.TextInput{State: inputState, CopyOverride: true, CopyRectangular: state.multiSelections[pane].rectangularClipboard(), CopyText: func() string { return clipboardSelection().clipboardText() }, OnEvent: func(event app.TextEvent) (app.TextState, error) {
		if event.Kind == app.TextCommandEvent && event.CompositionPhase == text.CompositionNone && state.moveMultiSelection(pane, event.Command) {
			return app.TextStateOf(doc.Editor), nil
		}
		if state.isReadOnly(doc) {
			state.cancelDocumentComposition(doc)
			state.notice = "Read-only document"
			return app.TextStateOf(doc.Editor), nil
		}
		if event.CompositionPhase != text.CompositionNone {
			if state.macroRecording {
				state.cancelMacroUnsupported("multi-selection composition")
			}
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
		cutting := event.Kind == app.TextPaste && value == "" && clipboardSelection().hasClipboardText()
		if event.Kind == app.TextCommandEvent {
			switch event.Command {
			case app.TextDeleteBackward:
				kind = multiDeleteBackward
			case app.TextDeleteForward:
				kind = multiDeleteForward
			case app.TextUndo, app.TextRedo:
				if state.macroRecording {
					state.cancelMacroUnsupported("undo or redo")
				}
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
		} else if event.Kind == app.TextPaste && !cutting {
			state.pasteMultiSelection(pane, value)
			return app.TextStateOf(doc.Editor), nil
		} else if !cutting && event.Kind != app.TextInsert {
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
