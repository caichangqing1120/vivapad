package main

import (
	"slices"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

type multiSelectionState struct {
	doc    *document.Document
	value  string
	ranges []text.Range
	active bool
}

func (state *model) syncMultiSelection(pane int, doc *document.Document) {
	selected := &state.multiSelections[pane]
	if selected.active && (selected.doc != doc || selected.value != doc.Editor.String() ||
		pane == state.activePane && len(selected.ranges) != 0 && doc.Editor.Selection() != selected.ranges[len(selected.ranges)-1]) {
		*selected = multiSelectionState{}
	}
}

func (state *model) multiSelectAll(options document.SearchOptions) {
	state.multiSelect(options, true)
}

func (state *model) multiSelectNext(options document.SearchOptions) {
	state.multiSelect(options, false)
}

func (state *model) multiSelect(options document.SearchOptions, all bool) {
	pane := state.activePane
	doc := state.paneDocument(pane)
	if _, composing := doc.Editor.Composition(); composing {
		return
	}
	state.multiSelectOptions = options
	state.syncMultiSelection(pane, doc)
	selected := &state.multiSelections[pane]
	if !selected.active {
		state.rectangles[pane] = rectangleSelection{}
		state.selectAnchors[pane] = selectAnchor{}
		initial := doc.Editor.Selection()
		initialWasEmpty := initial.Start == initial.End
		if initialWasEmpty {
			var found bool
			initial, found = doc.MultiSelectionWordAtCaret()
			if !found {
				return
			}
			doc.Editor.SetSelection(initial)
		}
		*selected = multiSelectionState{doc: doc, value: doc.Editor.String(), ranges: []text.Range{initial}, active: true}
		if !all && initialWasEmpty {
			return
		}
	}
	main := selected.ranges[len(selected.ranges)-1]
	if !options.Regex {
		if all {
			originalCount := len(selected.ranges)
			selected.ranges = doc.AppendLiteralSelections(main, selected.ranges, options)
			if len(selected.ranges) > originalCount {
				doc.Editor.SetSelection(selected.ranges[len(selected.ranges)-1])
			}
		} else if next, ok := doc.NextLiteralSelection(main, selected.ranges, options); ok {
			selected.ranges = append(selected.ranges, next)
			doc.Editor.SetSelection(next)
		}
		return
	}
	query := selected.value[doc.Editor.ByteOffset(main.Start):doc.Editor.ByteOffset(main.End)]
	matches, err := doc.MatchesWith(query, options)
	if err != nil || len(matches) == 0 {
		return
	}
	var after, before []text.Range
	for _, match := range matches {
		candidate := text.Range{Start: doc.Editor.RuneIndex(match.Start), End: doc.Editor.RuneIndex(match.End)}
		if candidate.Start >= main.End {
			after = append(after, candidate)
		} else if candidate.End <= main.Start {
			before = append(before, candidate)
		}
	}
	for _, candidate := range append(after, before...) {
		overlaps := false
		for _, existing := range selected.ranges {
			if candidate.Start < existing.End && candidate.End > existing.Start {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}
		selected.ranges = append(selected.ranges, candidate)
		doc.Editor.SetSelection(candidate)
		if !all {
			return
		}
	}
}

func (state *model) undoMultiSelect() {
	pane := state.activePane
	doc := state.paneDocument(pane)
	state.syncMultiSelection(pane, doc)
	selected := &state.multiSelections[pane]
	if !selected.active {
		return
	}
	selected.ranges = selected.ranges[:len(selected.ranges)-1]
	if len(selected.ranges) == 0 {
		*selected = multiSelectionState{}
		return
	}
	doc.Editor.SetSelection(selected.ranges[len(selected.ranges)-1])
}

func (state *model) skipMultiSelect() {
	pane := state.activePane
	doc := state.paneDocument(pane)
	state.syncMultiSelection(pane, doc)
	previous := len(state.multiSelections[pane].ranges)
	state.multiSelectNext(state.multiSelectOptions)
	selected := &state.multiSelections[pane]
	if previous == 0 || len(selected.ranges) <= previous {
		return
	}
	selected.ranges = slices.Delete(selected.ranges, previous-1, previous)
}

func (state *model) paintMultiSelection(snapshot *components.CodeEditorSnapshot, doc *document.Document, pane int) {
	state.syncMultiSelection(pane, doc)
	selected := state.multiSelections[pane]
	if !selected.active || len(selected.ranges) == 0 {
		return
	}
	ranges := append([]text.Range(nil), selected.ranges...)
	slices.SortFunc(ranges, func(left, right text.Range) int { return left.Start - right.Start })
	main := selected.ranges[len(selected.ranges)-1]
	snapshot.SelectionRanges, snapshot.ExtraCarets = nil, nil
	for _, selection := range ranges {
		if selection.Start == selection.End {
			if selection != main {
				snapshot.ExtraCarets = append(snapshot.ExtraCarets, components.CodeEditorCaret{Offset: doc.Editor.ByteOffset(selection.Start)})
			}
		} else {
			snapshot.SelectionRanges = append(snapshot.SelectionRanges, text.Span{
				Start: doc.Editor.ByteOffset(selection.Start), End: doc.Editor.ByteOffset(selection.End),
			})
		}
	}
	snapshot.Caret = doc.Editor.ByteOffset(main.End)
	snapshot.SelectionStart, snapshot.SelectionEnd = doc.Editor.ByteOffset(main.Start), snapshot.Caret
	snapshot.ShowCaret = main.Start == main.End
}
