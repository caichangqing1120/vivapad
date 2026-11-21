package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestLocaleRectangleMenuSortsKeysAndKeepsOwningViewHistory(t *testing.T) {
	for _, ascending := range []bool{false, true} {
		state, driver := startDriver(t)
		if err := driver.Resize(640, 400); err != nil {
			t.Fatal(err)
		}
		doc := state.space.Current()
		before := "outside keep\nz 22 keep\na 1 keep\nm 3 keep\ntail keep"
		want := "outside keep\na 1 keep\nm 3 keep\nz 22 keep\ntail keep"
		index := len(lineCommands) - 3
		if !ascending {
			want = "outside keep\nz 22 keep\nm 3 keep\na 1 keep\ntail keep"
			index++
		}
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		for style := range 5 {
			doc.Editor.SetSelection(text.Range{Start: 8, End: 12})
			state.styleToken(style, true)
		}
		state.query.Insert("keep")
		state.command("Mark All")
		state.command("Clone to Other View")
		state.activatePane(0)
		anchor := len("outside keep\nz ")
		caret := len("outside keep\nz 22 keep\na 1 keep\nm 3 ")
		doc.Editor.MoveTo(caret, false)
		state.rectangles[0] = rectangleSelection{doc: doc, anchor: anchor, caret: caret, value: before, active: true}
		previous := snapshotRectangle(state.rectangles[0])
		state.command("Line Operations")
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		for count := 0; count < len(lineCommands); count++ {
			found := false
			for _, hit := range driver.Snapshot().Hits {
				found = found || hit.ID == lineScope.At(index)
			}
			if found {
				break
			}
			clickID(t, driver, lineNextID)
		}
		clickID(t, driver, lineScope.At(index))
		if doc.Editor.String() != want || !state.rectangles[0].active || state.rectangles[1].active || state.rectangles[0].anchor == state.rectangles[0].caret {
			t.Fatalf("Locale rectangle menu lost scope/state: %q rect=%+v", doc.Editor.String(), state.rectangles[0])
		}
		after := snapshotRectangle(state.rectangles[0])
		state.syncFindMarks(doc)
		spans := []text.Span{{Start: 8, End: 12}, {Start: strings.LastIndex(want, "keep"), End: len(want)}}
		for style := range 5 {
			if !slices.Equal(state.styleSpans(doc, style), spans) {
				t.Fatalf("Locale restored replaced style %d: %+v", style, state.styleSpans(doc, style))
			}
		}
		if !slices.Equal(state.markedSpans, spans) {
			t.Fatalf("Locale restored replaced Find Mark: %+v", state.markedSpans)
		}
		state.activatePane(1)
		clickID(t, driver, rightEditorID)
		if _, err := driver.ShortcutChord(primary(), 'z'); err != nil || doc.Editor.String() != before || snapshotRectangle(state.rectangles[0]) != previous || state.rectangles[1].active || state.activePane != 1 || doc.Editor.CanUndo() {
			t.Fatalf("clone Locale Undo lost owner: %q %v", doc.Editor.String(), err)
		}
		if _, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'z'); err != nil || doc.Editor.String() != want || snapshotRectangle(state.rectangles[0]) != after || state.rectangles[1].active {
			t.Fatalf("clone Locale Redo lost owner: %q %v", doc.Editor.String(), err)
		}
	}
}

func TestLocaleRectangleEqualTextHasSavedHistoryAndNoStyleSnapshot(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	before := "z 1 keep\na 2 keep"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	if err := doc.Save(filepath.Join(t.TempDir(), "locale-equal.txt")); err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(0, true)
	state.query.Insert("keep")
	state.command("Mark All")
	caret := len("z 1 keep\na 2")
	doc.Editor.MoveTo(caret, false)
	state.rectangles[0] = rectangleSelection{doc: doc, anchor: 2, caret: caret, value: before, active: true}
	previous := snapshotRectangle(state.rectangles[0])
	state.command("Sort In Locale Order Ascending")
	state.syncFindMarks(doc)
	if doc.Editor.String() != before || !doc.Dirty() || !doc.Editor.CanUndo() || len(state.styleSpans(doc, 0)) != 0 || len(state.markedSpans) != 0 || snapshotRectangle(state.rectangles[0]) != previous {
		t.Fatalf("equal Locale was skipped or lost rectangle: %q rect=%+v", doc.Editor.String(), state.rectangles[0])
	}
	clickID(t, driver, undoID)
	if doc.Dirty() || doc.Editor.CanUndo() || snapshotRectangle(state.rectangles[0]) != previous || len(state.styleSpans(doc, 0)) != 0 {
		t.Fatal("equal Locale Undo lost saved/rectangle state or restored a style snapshot")
	}
	clickID(t, driver, redoID)
	if !doc.Dirty() || snapshotRectangle(state.rectangles[0]) != previous {
		t.Fatal("equal Locale Redo lost saved/rectangle state")
	}
}

func TestLocaleRectangleSingleRowAndRefusalKeepState(t *testing.T) {
	for _, kind := range []string{"single", "read-only", "composition", "restriction"} {
		state, _ := startDriver(t)
		doc := state.space.Current()
		before := "z 2\na 1"
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		caret := len(before)
		if kind == "single" {
			caret = 2
		}
		doc.Editor.MoveTo(caret, false)
		state.rectangles[0] = rectangleSelection{doc: doc, anchor: 2, caret: caret, value: before, active: true, composing: kind == "composition"}
		state.readOnly[doc] = kind == "read-only"
		if kind == "restriction" {
			doc.Editor.Restrict(func(rune) bool { return true }, 0)
		}
		previous := snapshotRectangle(state.rectangles[0])
		state.command("Sort In Locale Order Ascending")
		if doc.Editor.String() != before || doc.Editor.Caret() != caret || doc.Editor.CanUndo() || snapshotRectangle(state.rectangles[0]) != previous {
			t.Fatalf("%s Locale changed source/view state", kind)
		}
	}
}
