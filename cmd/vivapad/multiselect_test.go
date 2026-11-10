package main

import (
	"reflect"
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
