package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func lineKeepSpans(value string) []text.Span {
	var spans []text.Span
	for position := 0; position < len(value); {
		found := strings.Index(value[position:], "keep")
		if found < 0 {
			break
		}
		position += found
		spans = append(spans, text.Span{Start: position, End: position + 4})
		position += 4
	}
	return spans
}

func TestJoinSplitSourceTargetsKeepFiveStylesAndFindMark(t *testing.T) {
	for _, command := range []string{"Join Lines", "Split Lines"} {
		t.Run(command, func(t *testing.T) {
			state, driver := startDriver(t)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			before := "left keep\nright keep\nfooter keep"
			if command == "Split Lines" {
				before = strings.Repeat("alpha keep beta ", 8) + "\nfooter keep"
			}
			doc.Editor.Insert(before)
			doc.Editor.ClearHistory()
			first := strings.Index(before, "keep")
			for style := range 5 {
				doc.Editor.SetSelection(text.Range{Start: first, End: first + 4})
				state.styleToken(style, true)
			}
			state.query.Insert("keep")
			state.command("Mark All")
			if command == "Join Lines" {
				doc.Editor.SetSelection(text.Range{Start: len("left keep\nright keep\n"), End: 0})
			} else {
				doc.Editor.MoveTo(0, false)
			}
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			state.command(command)
			after := doc.Editor.String()
			if after == before {
				t.Fatal("line fixture made no source change")
			}
			want := lineKeepSpans(after)
			state.syncFindMarks(doc)
			if len(want) != len(lineKeepSpans(before)) || !slices.Equal(state.markedSpans, want) {
				t.Fatalf("%s lost untouched Find Mark: %+v want %+v", command, state.markedSpans, want)
			}
			for style := range 5 {
				if !slices.Equal(state.styleSpans(doc, style), want) {
					t.Fatalf("%s lost untouched style %d", command, style)
				}
			}
			selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
			clickID(t, driver, undoID)
			state.syncFindMarks(doc)
			if doc.Editor.String() != before || doc.Editor.CanUndo() || !slices.Equal(state.styleSpans(doc, 0), lineKeepSpans(before)) || !slices.Equal(state.markedSpans, lineKeepSpans(before)) {
				t.Fatal("line Undo replaced untouched body ranges")
			}
			clickID(t, driver, redoID)
			if doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || !slices.Equal(state.styleSpans(doc, 4), want) {
				t.Fatal("line Redo lost source final selection or styles")
			}
		})
	}
}

func TestLineDeletionKeepsUntouchedBodyRangesAndSourceEOFCopy(t *testing.T) {
	for _, command := range []string{"Remove Empty Lines", "Remove Blank Lines"} {
		state, driver := startDriver(t)
		doc := state.space.Current()
		before := "head keep\n\nmiddle keep\n"
		if command == "Remove Blank Lines" {
			before = "head keep\n \t\nmiddle keep\n"
		}
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(text.Range{Start: 5, End: 9})
		state.styleToken(0, true)
		state.query.Insert("keep")
		state.command("Mark All")
		doc.Editor.MoveTo(0, false)
		state.command(command)
		after := "head keep\nmiddle keep"
		state.syncFindMarks(doc)
		if doc.Editor.String() != after || !slices.Equal(state.styleSpans(doc, 0), lineKeepSpans(after)) || !slices.Equal(state.markedSpans, lineKeepSpans(after)) {
			t.Fatal("empty/blank deletion replaced untouched bodies")
		}
		clickID(t, driver, undoID)
		if doc.Editor.String() != before || doc.Editor.CanUndo() || !slices.Equal(state.styleSpans(doc, 0), lineKeepSpans(before)) {
			t.Fatal("empty/blank deletion was not one primitive-preserving Undo")
		}
	}
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("keep\nkeep\nkeep")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 4})
	state.styleToken(1, false)
	doc.Editor.SetSelection(text.Range{Start: 10, End: 14})
	state.styleToken(0, false)
	state.query.Insert("keep")
	state.command("Mark All")
	doc.Editor.MoveTo(0, false)
	state.command("Remove Consecutive Duplicates")
	state.syncFindMarks(doc)
	if doc.Editor.String() != "keep" || len(state.styleSpans(doc, 1)) != 0 || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 0, End: 4}}) || !slices.Equal(state.markedSpans, []text.Span{{Start: 0, End: 4}}) {
		t.Fatal("consecutive EOF deletion kept the wrong source copy or restored a style snapshot")
	}
	clickID(t, driver, undoID)
	state.syncFindMarks(doc)
	if doc.Editor.String() != "keep\nkeep\nkeep" || doc.Editor.CanUndo() || len(state.styleSpans(doc, 1)) != 0 || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 10, End: 14}}) || !slices.Equal(state.markedSpans, []text.Span{{Start: 10, End: 14}}) {
		t.Fatal("consecutive EOF Undo recreated deleted source indicators")
	}
}

func TestSortAndAnyDuplicateKeepOriginalWholeBlockStylePolicy(t *testing.T) {
	for _, command := range []string{"Sort Lines Ascending", "Remove Duplicate Lines"} {
		state, driver := startDriver(t)
		doc := state.space.Current()
		before := "outside keep\nz keep\na keep\ntail keep"
		end := len("outside keep\nz keep\na keep\n")
		if command == "Remove Duplicate Lines" {
			before = "outside keep\na keep\nb keep\na keep\ntail keep"
			end = len("outside keep\na keep\nb keep\na keep\n")
		}
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(text.Range{Start: 8, End: 12})
		state.styleToken(0, true)
		state.query.Insert("keep")
		state.command("Mark All")
		doc.Editor.SetSelection(text.Range{Start: len("outside keep\n"), End: end})
		state.command(command)
		after := doc.Editor.String()
		want := []text.Span{{Start: 8, End: 12}, {Start: strings.LastIndex(after, "keep"), End: len(after)}}
		state.syncFindMarks(doc)
		if after == before || !slices.Equal(state.styleSpans(doc, 0), want) || !slices.Equal(state.markedSpans, want) {
			t.Fatalf("%s did not retain original full-target semantics: text=%q styles=%+v marks=%+v", command, after, state.styleSpans(doc, 0), state.markedSpans)
		}
		clickID(t, driver, undoID)
		beforeWant := []text.Span{{Start: 8, End: 12}, {Start: strings.LastIndex(before, "keep"), End: len(before)}}
		if doc.Editor.String() != before || !slices.Equal(state.styleSpans(doc, 0), beforeWant) {
			t.Fatal("whole-block Undo invented an indicator snapshot")
		}
	}
}

func TestSplitLinesRefusesMultipleSourceSelections(t *testing.T) {
	for _, rectangle := range []bool{false, true} {
		state, driver := startDriver(t)
		if err := driver.Resize(640, 400); err != nil {
			t.Fatal(err)
		}
		doc := state.space.Current()
		before := strings.Repeat("alpha keep beta ", 8) + "\nnext keep"
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		ranges := []text.Range{{Start: 0, End: 4}, {Start: 5, End: 10}}
		doc.Editor.SetSelection(ranges[1])
		if rectangle {
			state.rectangles[0] = rectangleSelection{doc: doc, anchor: 0, caret: len(before) - 1, value: before, active: true}
		} else {
			state.multiSelections[0] = multiSelectionState{doc: doc, value: before, ranges: ranges, active: true}
		}
		selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
		state.command("Split Lines")
		if doc.Editor.String() != before || doc.Editor.CanUndo() || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret {
			t.Fatalf("rectangle=%v: Split silently reduced multiple selections to one", rectangle)
		}
	}
}

func TestClassicAndLocaleSortRefuseMultipleStreamSelections(t *testing.T) {
	for _, command := range []string{"Sort Lines Ascending", "Sort As Integers Ascending", "Sort In Locale Order Ascending"} {
		state, driver := startDriver(t)
		doc := state.space.Current()
		before := "z10 keep\na2 keep\noutside"
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		ranges := []text.Range{{Start: 0, End: 1}, {Start: 2, End: len("z10 keep\na2 keep\n")}}
		doc.Editor.SetSelection(ranges[1])
		state.multiSelections[0] = multiSelectionState{doc: doc, value: before, ranges: append([]text.Range(nil), ranges...), active: true}
		state.command(command)
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		if doc.Editor.String() != before || doc.Editor.CanUndo() || doc.Editor.Selection() != ranges[1] || !slices.Equal(state.multiSelections[0].ranges, ranges) {
			t.Fatalf("%s edited multiple stream selections", command)
		}
	}
}

func TestNewLineCommandsDoNotReviveClearedBookmarksWhenBytesReturn(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("b\na")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.toggleBookmark()
	state.command("Sort Lines Ascending")
	if len(state.bookmarkLines(doc)) != 0 {
		t.Fatal("whole-buffer sort retained markers cleared by original CellBuffer")
	}
	state.command("Sort Lines Descending")
	if doc.Editor.String() != "b\na" || len(state.bookmarkLines(doc)) != 0 {
		t.Fatal("a new edit returning to equal bytes resurrected a bookmark snapshot")
	}
}

func TestEqualLocaleSortAppliesRealIndicatorAndBookmarkMutation(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("a keep\nb keep\n")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 2, End: 6})
	state.styleToken(0, true)
	state.query.Insert("keep")
	state.command("Mark All")
	doc.Editor.MoveTo(0, false)
	state.toggleBookmark()
	before := doc.Editor.String()
	state.command("Sort In Locale Order Ascending")
	state.syncFindMarks(doc)
	if doc.Editor.String() != before || !doc.Editor.CanUndo() || len(state.styleSpans(doc, 0)) != 0 || len(state.markedSpans) != 0 || len(state.bookmarkLines(doc)) != 0 {
		t.Fatal("equal Locale sort skipped the source full-target mutations")
	}
	clickID(t, driver, undoID)
	if doc.Editor.String() != before || doc.Editor.CanUndo() || len(state.styleSpans(doc, 0)) != 0 {
		t.Fatal("equal Locale Undo recreated deleted indicator snapshots")
	}
}
