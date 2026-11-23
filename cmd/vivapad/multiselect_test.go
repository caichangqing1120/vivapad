package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestMultiSelectAllExpandsWordBeforeAddingEveryOccurrence(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("one ONE one\none")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	state.multiSelectAll(document.SearchOptions{})
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{{Start: 0, End: 3}}) {
		t.Fatalf("first All should select the caret word: %+v", got)
	}
	state.multiSelectAll(document.SearchOptions{})
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{
		{Start: 0, End: 3}, {Start: 4, End: 7}, {Start: 8, End: 11}, {Start: 12, End: 15},
	}) || doc.Editor.String() != "one ONE one\none" || doc.Editor.CanUndo() {
		t.Fatalf("All selections = %+v, text=%q", got, doc.Editor.String())
	}
}

func TestMultiSelectAllKeepsExistingRangesAndWrapOrder(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	const value = "cat cat cat cat cat cat"
	doc.Editor.Insert(value)
	doc.Editor.ClearHistory()
	existing := []text.Range{{Start: 16, End: 19}, {Start: 4, End: 7}, {Start: 12, End: 15}}
	doc.Editor.SetSelection(existing[len(existing)-1])
	state.multiSelections[0] = multiSelectionState{doc: doc, value: value, ranges: existing, active: true}
	want := []text.Range{
		{Start: 16, End: 19}, {Start: 4, End: 7}, {Start: 12, End: 15},
		{Start: 20, End: 23}, {Start: 0, End: 3}, {Start: 8, End: 11},
	}
	for range 2 {
		state.multiSelectAll(document.SearchOptions{WholeWord: true})
		if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, want) || doc.Editor.Selection() != want[len(want)-1] {
			t.Fatalf("All selected %+v, main %+v; want %+v", got, doc.Editor.Selection(), want)
		}
	}
	state.undoMultiSelect()
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, want[:len(want)-1]) || doc.Editor.Selection() != want[len(want)-2] {
		t.Fatalf("Undo Latest selected %+v, main %+v", got, doc.Editor.Selection())
	}
	if doc.Editor.String() != value || doc.Editor.CanUndo() {
		t.Fatal("selection changed text or created an edit Undo entry")
	}
}

func TestMultiSelectAllAvoidsAnAllocationPerMatch(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	const occurrences = 10_000
	doc.Editor.Insert(strings.Repeat("cat ", occurrences))
	doc.Editor.ClearHistory()
	first := text.Range{Start: 0, End: 3}
	last := text.Range{Start: (occurrences - 1) * 4, End: occurrences*4 - 1}
	allocs := testing.AllocsPerRun(3, func() {
		state.multiSelections[0] = multiSelectionState{}
		doc.Editor.SetSelection(first)
		state.multiSelectAll(document.SearchOptions{WholeWord: true})
		got := state.multiSelections[0].ranges
		if len(got) != occurrences || got[0] != first || got[len(got)-1] != last || doc.Editor.Selection() != last || doc.Editor.CanUndo() {
			t.Fatalf("All selected %d ranges, main %+v; want %d ranges, main %+v, and no edit", len(got), doc.Editor.Selection(), occurrences, last)
		}
	})
	t.Logf("allocations per Select All: %.0f", allocs)
	if allocs > 256 {
		t.Fatalf("All allocated %.0f times for %d matches", allocs, occurrences)
	}
}

func TestMultiSelectNextUndoLatestSkipAndWrap(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("red red red red")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectNext(document.SearchOptions{})
	state.multiSelectNext(document.SearchOptions{})
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{
		{Start: 0, End: 3}, {Start: 4, End: 7}, {Start: 8, End: 11},
	}) {
		t.Fatalf("Next selections = %+v", got)
	}
	state.undoMultiSelect()
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{{Start: 0, End: 3}, {Start: 4, End: 7}}) {
		t.Fatalf("Undo latest selection = %+v", got)
	}
	state.skipMultiSelect()
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{{Start: 0, End: 3}, {Start: 8, End: 11}}) {
		t.Fatalf("Skip current = %+v", got)
	}
	state.multiSelectNext(document.SearchOptions{})
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{
		{Start: 0, End: 3}, {Start: 8, End: 11}, {Start: 12, End: 15},
	}) {
		t.Fatalf("Next after skip = %+v", got)
	}
	state.multiSelectNext(document.SearchOptions{})
	if len(state.multiSelections[0].ranges) != 4 || state.multiSelections[0].ranges[3] != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("wrapped Next should revisit previously skipped match: %+v", state.multiSelections[0].ranges)
	}
}

func TestMultiSelectNextSkipsMatchesOverlappingTheInitialRange(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("aaaaaa")
	doc.Editor.ClearHistory()
	initial := text.Range{Start: 1, End: 3}
	doc.Editor.SetSelection(initial)
	state.multiSelectNext(document.SearchOptions{MatchCase: true})
	state.multiSelectNext(document.SearchOptions{MatchCase: true})
	want := []text.Range{initial, {Start: 4, End: 6}}
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, want) || doc.Editor.CanUndo() {
		t.Fatalf("selected %+v, want %+v with no Undo entry", got, want)
	}
}

func TestMultiSelectNextDoesNotAllocateForOtherMatches(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert(strings.Repeat("cat ", 50_000))
	doc.Editor.ClearHistory()
	allocations := testing.AllocsPerRun(3, func() {
		state.multiSelections[0] = multiSelectionState{}
		doc.Editor.MoveTo(1, false)
		state.multiSelectNext(document.SearchOptions{WholeWord: true})
		state.multiSelectNext(document.SearchOptions{WholeWord: true})
		if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{{Start: 0, End: 3}, {Start: 4, End: 7}}) || doc.Editor.CanUndo() {
			t.Fatalf("selected %+v, wanted the first two matches without editing", got)
		}
	})
	t.Logf("allocations per initial selection and Next: %.0f", allocations)
	if allocations > 256 {
		t.Fatalf("Next allocated %.0f times for 50,000 matches", allocations)
	}
}

func TestMultiSelectMatchFlagsAndUnicodeWord(t *testing.T) {
	for _, option := range []struct {
		flags document.SearchOptions
		count int
	}{
		{document.SearchOptions{}, 4},
		{document.SearchOptions{MatchCase: true}, 3},
		{document.SearchOptions{WholeWord: true}, 3},
		{document.SearchOptions{MatchCase: true, WholeWord: true}, 2},
	} {
		state := newModel()
		doc := state.space.Current()
		doc.Editor.Insert("cat cats CAT cat")
		doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
		state.multiSelectAll(option.flags)
		if got := len(state.multiSelections[0].ranges); got != option.count {
			t.Fatalf("flags %+v selected %d, want %d", option.flags, got, option.count)
		}
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("猫_22 猫_22")
	doc.Editor.MoveTo(2, false)
	state.multiSelectNext(document.SearchOptions{WholeWord: true})
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{{Start: 0, End: 4}}) {
		t.Fatalf("Unicode word at caret = %+v", got)
	}
	state.multiSelectNext(document.SearchOptions{WholeWord: true})
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{{Start: 0, End: 4}, {Start: 5, End: 9}}) {
		t.Fatalf("Unicode Next = %+v", got)
	}
}

func TestMultiSelectNoWordAndChangedDocumentCannotKeepStaleRanges(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert(".alpha alpha")
	doc.Editor.MoveTo(0, false)
	state.multiSelectAll(document.SearchOptions{})
	if state.multiSelections[0].active || doc.Editor.String() != ".alpha alpha" {
		t.Fatal("punctuation at caret created a selection")
	}
	doc.Editor.SetSelection(text.Range{Start: 1, End: 6})
	state.multiSelectNext(document.SearchOptions{})
	if len(state.multiSelections[0].ranges) != 2 {
		t.Fatal("expected two selected words")
	}
	doc.Editor.MoveTo(0, false)
	doc.Editor.Insert("changed ")
	state.syncMultiSelection(0, doc)
	if state.multiSelections[0].active || len(state.multiSelections[0].ranges) != 0 {
		t.Fatalf("stale multi-selection survived external edit: %+v", state.multiSelections[0])
	}
}
