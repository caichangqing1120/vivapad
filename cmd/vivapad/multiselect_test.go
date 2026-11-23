package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestMultiSelectFirstInvocationFromCaret(t *testing.T) {
	const value = "cat CAT cat cats"
	seed := text.Range{Start: 8, End: 11}
	after := text.Range{Start: 12, End: 15}
	first := text.Range{Start: 0, End: 3}
	upper := text.Range{Start: 4, End: 7}
	for _, tc := range []struct {
		name  string
		flags document.SearchOptions
		want  []text.Range
	}{
		{"unrestricted", document.SearchOptions{}, []text.Range{seed, after, first, upper}},
		{"match case", document.SearchOptions{MatchCase: true}, []text.Range{seed, after, first}},
		{"whole word", document.SearchOptions{WholeWord: true}, []text.Range{seed, first, upper}},
		{"match case and whole word", document.SearchOptions{MatchCase: true, WholeWord: true}, []text.Range{seed, first}},
	} {
		for _, all := range []bool{true, false} {
			mode := "Next"
			want := []text.Range{seed}
			if all {
				mode, want = "All", tc.want
			}
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				state := newModel()
				doc := state.space.Current()
				doc.Editor.Insert(value)
				doc.Editor.ClearHistory()
				doc.Editor.MoveTo(9, false)
				state.multiSelect(tc.flags, all)
				if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, want) || doc.Editor.Selection() != want[len(want)-1] {
					t.Fatalf("selected %+v, main %+v; want ranges %+v", got, doc.Editor.Selection(), want)
				}
				if doc.Editor.String() != value || doc.Editor.CanUndo() {
					t.Fatal("multi-select changed the text or edit history")
				}
			})
		}
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

func TestMultiSelectResumesFromCaretsAfterBatchEdit(t *testing.T) {
	for _, mode := range []string{"Next", "All", "Next then All", "Skip at a covered caret", "no word"} {
		t.Run(mode, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert("one one new")
			doc.Editor.ClearHistory()
			first := text.Range{Start: 0, End: 3}
			second := text.Range{Start: 4, End: 7}
			doc.Editor.SetSelection(first)
			options := document.SearchOptions{WholeWord: true}
			state.multiSelectAll(options)
			replacement := "new"
			if mode == "no word" {
				replacement = "!"
			}
			if changed, err := state.editMultiSelection(0, replacement, multiInsert); err != nil || !changed {
				t.Fatalf("replace: changed=%t, error=%v", changed, err)
			}
			value := doc.Editor.String()
			want := append([]text.Range(nil), state.multiSelections[0].ranges...)
			switch mode {
			case "Next":
				state.multiSelectNext(options)
				want = []text.Range{{Start: 3, End: 3}, second}
			case "All", "Next then All":
				if mode == "Next then All" {
					state.multiSelectNext(options)
				}
				state.multiSelectAll(options)
				want = []text.Range{second, {Start: 8, End: 11}, first}
			case "Skip at a covered caret":
				state.multiSelectNext(options)
				state.multiSelectNext(options)
				state.skipMultiSelect()
				want = []text.Range{second, first}
			default:
				state.multiSelectNext(options)
				state.multiSelectAll(options)
			}
			if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, want) || doc.Editor.Selection() != want[len(want)-1] || doc.Editor.String() != value {
				t.Fatalf("ranges = %+v, main = %+v, text = %q; want %+v and %q", got, doc.Editor.Selection(), doc.Editor.String(), want, value)
			}
			state.command("Undo")
			if doc.Editor.String() != "one one new" || doc.Editor.CanUndo() || !reflect.DeepEqual(state.multiSelections[0].ranges, []text.Range{first, second}) {
				t.Fatalf("undo = %q, ranges=%+v, history=%t", doc.Editor.String(), state.multiSelections[0].ranges, doc.Editor.CanUndo())
			}
		})
	}
}

func TestMultiSelectFromEmptyMainInMixedSelection(t *testing.T) {
	old := text.Range{Start: 0, End: 3}
	partial := text.Range{Start: 4, End: 6}
	caret := text.Range{Start: 6, End: 6}
	unrelated := text.Range{Start: 16, End: 16}
	for _, tc := range []struct {
		name, value string
		all         bool
		initial     []text.Range
		want        []text.Range
	}{
		{"expand and find matches", "old new old new!", true, []text.Range{old, unrelated, caret}, []text.Range{old, unrelated, {Start: 4, End: 7}, {Start: 12, End: 15}}},
		{"next keeps mixed empty query", "old new old new!", false, []text.Range{old, unrelated, caret}, []text.Range{old, unrelated, caret}},
		{"no word", "old . ", true, []text.Range{old, {Start: 5, End: 5}}, []text.Range{old, {Start: 5, End: 5}}},
		{"word overlaps a range", "old new", true, []text.Range{partial, {Start: 7, End: 7}}, []text.Range{partial, {Start: 7, End: 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert(tc.value)
			doc.Editor.ClearHistory()
			doc.Editor.SetSelection(tc.initial[len(tc.initial)-1])
			state.multiSelections[0] = multiSelectionState{doc: doc, active: true, value: tc.value, ranges: append([]text.Range(nil), tc.initial...)}
			state.multiSelect(document.SearchOptions{WholeWord: true}, tc.all)
			if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, tc.want) || doc.Editor.Selection() != tc.want[len(tc.want)-1] {
				t.Fatalf("ranges = %+v, main = %+v; want %+v", got, doc.Editor.Selection(), tc.want)
			}
			if doc.Editor.String() != tc.value || doc.Editor.CanUndo() {
				t.Fatal("multi-select changed text or edit history")
			}
		})
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

func TestMultiSelectSearchesFromSelectionBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		options     document.SearchOptions
		initial     text.Range
		want        []text.Range
	}{
		{"adjacent match", "aaaaaa", document.SearchOptions{MatchCase: true}, text.Range{Start: 1, End: 3}, []text.Range{{Start: 3, End: 5}}},
		{"forward then wrap", "aaaaaaaaaaaa", document.SearchOptions{MatchCase: true}, text.Range{Start: 3, End: 5}, []text.Range{{Start: 5, End: 7}, {Start: 7, End: 9}, {Start: 9, End: 11}, {Start: 0, End: 2}}},
		{"folded variable width", "\u212ak\u212ak\u212ak", document.SearchOptions{}, text.Range{Start: 1, End: 3}, []text.Range{{Start: 3, End: 5}}},
		{"whole word sees past the search region", "aa aaaa aa", document.SearchOptions{WholeWord: true}, text.Range{Start: 3, End: 5}, []text.Range{{Start: 8, End: 10}, {Start: 0, End: 2}}},
	} {
		for _, all := range []bool{false, true} {
			mode, matches := "Next", tc.want[:1]
			if all {
				mode, matches = "All", tc.want
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				state := newModel()
				doc := state.space.Current()
				doc.Editor.Insert(tc.value)
				doc.Editor.ClearHistory()
				doc.Editor.SetSelection(tc.initial)
				state.multiSelect(tc.options, all)
				want := append([]text.Range{tc.initial}, matches...)
				if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, want) || doc.Editor.Selection() != want[len(want)-1] || doc.Editor.CanUndo() || doc.Editor.String() != tc.value {
					t.Fatalf("selected %+v, main %+v, want %+v without editing", got, doc.Editor.Selection(), want)
				}
			})
		}
	}
}

func TestMultiSelectSearchContinuesAfterAnOccupiedCandidate(t *testing.T) {
	const value = "aaaaaa"
	for _, all := range []bool{false, true} {
		state := newModel()
		doc := state.space.Current()
		doc.Editor.Insert(value)
		doc.Editor.ClearHistory()
		initial := text.Range{Start: 1, End: 3}
		occupied := text.Range{Start: 3, End: 4}
		doc.Editor.SetSelection(initial)
		state.multiSelections[0] = multiSelectionState{doc: doc, value: value, active: true, ranges: []text.Range{occupied, initial}}
		state.multiSelect(document.SearchOptions{}, all)
		want := []text.Range{occupied, initial, {Start: 4, End: 6}}
		if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, want) || doc.Editor.CanUndo() {
			t.Fatalf("All=%t: selected %+v, want %+v without editing", all, got, want)
		}
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

func TestMultiSelectScintillaUTF8WordBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		initial     text.Range
		want        []text.Range
	}{
		{"combining mark at caret", "e\u0301 x", text.Range{Start: 2, End: 2}, []text.Range{{Start: 0, End: 2}}},
		{"number at caret", "x\u00b2 z", text.Range{Start: 2, End: 2}, []text.Range{{Start: 0, End: 2}}},
		{"combining mark excludes partial match", "e e\u0301 e", text.Range{Start: 0, End: 1}, []text.Range{{Start: 0, End: 1}, {Start: 5, End: 6}}},
		{"punctuation respects class edges", ": :: x:y :", text.Range{Start: 0, End: 1}, []text.Range{{Start: 0, End: 1}, {Start: 6, End: 7}, {Start: 9, End: 10}}},
	} {
		for _, all := range []bool{false, true} {
			mode := "Next"
			if all {
				mode = "All"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				state := newModel()
				doc := state.space.Current()
				doc.Editor.Insert(tc.value)
				doc.Editor.ClearHistory()
				doc.Editor.SetSelection(tc.initial)
				state.multiSelect(document.SearchOptions{WholeWord: true}, all)
				want := tc.want
				if !all && len(want) > 2 {
					want = want[:2]
				}
				if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, want) || doc.Editor.Selection() != want[len(want)-1] {
					t.Fatalf("ranges = %+v, main = %+v; want %+v", got, doc.Editor.Selection(), want)
				}
				if doc.Editor.String() != tc.value || doc.Editor.CanUndo() {
					t.Fatal("multi-selection changed text or edit history")
				}
			})
		}
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
