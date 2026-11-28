package main

import (
	"cmp"
	"slices"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
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

func (state *model) exitMultiSelection(pane int) bool {
	doc := state.paneDocument(pane)
	state.syncMultiSelection(pane, doc)
	rectangle := state.rectangles[pane]
	if doc.Editor.CompositionActive() || !state.multiSelections[pane].active &&
		(rectangle.doc != doc || rectangle.value != doc.Editor.String() || !rectangle.active && !rectangle.pending) {
		return false
	}
	state.multiSelections[pane] = multiSelectionState{}
	state.rectangles[pane] = rectangleSelection{}
	state.altEditorGesture = altEditorGesture{}
	doc.Editor.MoveTo(doc.Editor.Caret(), false)
	if state.macroRecording {
		state.cancelMacroUnsupported("exit multi-selection")
	}
	return true
}

func multiMovementForKey(key gfx.Key) app.TextCommand {
	switch key {
	case gfx.KeyLeft:
		return app.TextMoveLeft
	case gfx.KeyRight:
		return app.TextMoveRight
	case gfx.KeyHome:
		return app.TextMoveLineStart
	case gfx.KeyEnd:
		return app.TextMoveLineEnd
	}
	return 0
}

func (state *model) moveMultiSelection(pane int, movement app.TextCommand) bool {
	switch movement {
	case app.TextMoveLeft, app.TextMoveRight, app.TextMoveLineStart, app.TextMoveLineEnd:
	default:
		return false
	}
	doc := state.paneDocument(pane)
	state.syncMultiSelection(pane, doc)
	selected := &state.multiSelections[pane]
	if !selected.active || doc.Editor.CompositionActive() {
		return false
	}
	carets := make([]text.Range, 0, len(selected.ranges))
	mainCaret := doc.Editor.Caret()
	for index, selection := range selected.ranges {
		if index == len(selected.ranges)-1 && mainCaret == selection.Start {
			selection.Start, selection.End = selection.End, selection.Start
		}
		doc.Editor.SetSelection(selection)
		switch movement {
		case app.TextMoveLeft:
			doc.Editor.MoveLeft(false)
		case app.TextMoveRight:
			doc.Editor.MoveRight(false)
		case app.TextMoveLineStart, app.TextMoveLineEnd:
			doc.Editor.MoveLineBoundary(movement == app.TextMoveLineEnd, false)
		}
		carets = append(carets, doc.Editor.Selection())
	}
	selected.ranges = uniqueMultiCarets(carets)
	if state.macroRecording {
		state.cancelMacroUnsupported("multi-selection movement")
	}
	return true
}

func uniqueMultiCarets(carets []text.Range) []text.Range {
	seen := make(map[text.Range]struct{}, len(carets))
	slices.Reverse(carets)
	carets = slices.DeleteFunc(carets, func(caret text.Range) bool {
		_, duplicate := seen[caret]
		seen[caret] = struct{}{}
		return duplicate
	})
	slices.Reverse(carets)
	return carets
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
	if main.Start == main.End {
		if !all && slices.ContainsFunc(selected.ranges, func(r text.Range) bool { return r.Start != r.End }) {
			return
		}
		word, found := doc.MultiSelectionWordAtCaret()
		if !found || slices.ContainsFunc(selected.ranges, func(r text.Range) bool {
			return r.Start != r.End && r.Start < word.End && word.Start < r.End
		}) {
			return
		}
		main = word
		selected.ranges[len(selected.ranges)-1] = word
		selected.ranges = trimCoveredMultiSelectCarets(selected.ranges)
		doc.Editor.SetSelection(word)
		if !all {
			return
		}
	}
	if !options.Regex {
		if all {
			originalCount := len(selected.ranges)
			selected.ranges = doc.AppendLiteralSelections(main, selected.ranges, options)
			if len(selected.ranges) > originalCount {
				selected.ranges = trimCoveredMultiSelectCarets(selected.ranges)
				doc.Editor.SetSelection(selected.ranges[len(selected.ranges)-1])
			}
		} else if next, ok := doc.NextLiteralSelection(main, selected.ranges, options); ok {
			selected.ranges = trimCoveredMultiSelectCarets(append(selected.ranges, next))
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
	added := false
	for _, candidate := range append(after, before...) {
		overlaps := false
		for _, existing := range selected.ranges {
			if existing.Start != existing.End && candidate.Start < existing.End && candidate.End > existing.Start {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}
		selected.ranges = append(selected.ranges, candidate)
		added = true
		doc.Editor.SetSelection(candidate)
		if !all {
			break
		}
	}
	if added {
		selected.ranges = trimCoveredMultiSelectCarets(selected.ranges)
	}
}

func trimCoveredMultiSelectCarets(ranges []text.Range) []text.Range {
	if !slices.ContainsFunc(ranges, func(r text.Range) bool { return r.Start == r.End }) {
		return ranges
	}
	words := make([]text.Range, 0, len(ranges))
	for _, r := range ranges {
		if r.Start != r.End {
			words = append(words, r)
		}
	}
	slices.SortFunc(words, func(a, b text.Range) int { return cmp.Compare(a.End, b.End) })
	return slices.DeleteFunc(ranges, func(r text.Range) bool {
		if r.Start != r.End {
			return false
		}
		i, _ := slices.BinarySearchFunc(words, r.Start, func(word text.Range, at int) int { return cmp.Compare(word.End, at) })
		return i < len(words) && words[i].Start <= r.Start
	})
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
	selected := &state.multiSelections[pane]
	var previous text.Range
	if len(selected.ranges) != 0 {
		previous = selected.ranges[len(selected.ranges)-1]
	}
	state.multiSelectNext(state.multiSelectOptions)
	if previous.Start == previous.End || len(selected.ranges) == 0 || selected.ranges[len(selected.ranges)-1] == previous {
		return
	}
	if i := slices.Index(selected.ranges, previous); i >= 0 {
		selected.ranges = slices.Delete(selected.ranges, i, i+1)
	}
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
