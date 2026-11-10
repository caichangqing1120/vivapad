package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestLocaleAndRandomSortCommandsReachableAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("item10\nitem2\nitem1\n")
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	for step := 0; step < len(lineCommands); step++ {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == lineScope.At(len(lineCommands)-1) {
				found = true
			}
		}
		if found {
			break
		}
		clickID(t, driver, lineNextID)
	}
	for _, index := range []int{len(lineCommands) - 3, len(lineCommands) - 2, len(lineCommands) - 1} {
		seen := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == lineScope.At(index) {
				seen = true
				if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("new sort row clipped: %+v", hit.Bounds)
				}
			}
		}
		if !seen {
			t.Fatalf("sort menu item %d unavailable", index)
		}
	}
	clickID(t, driver, lineScope.At(len(lineCommands)-3))
	if got := doc.Editor.String(); got != "item1\nitem2\nitem10\n" {
		t.Fatalf("locale sort menu = %q", got)
	}
	state.command("Read-Only on Current Document")
	state.command("Randomize Line Order")
	if doc.Editor.String() != "item1\nitem2\nitem10\n" || state.notice != "Read-only document" {
		t.Fatalf("read-only random sort = %q notice=%q", doc.Editor.String(), state.notice)
	}
}

func TestLocaleSortClearsSourceWholeBufferMarkersAndKeepsActiveView(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("item10\nitem2\nitem1\n")
	right.Editor.ClearHistory()
	right.Editor.MoveTo(0, false)
	state.toggleBookmark()
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[right] = 1
	clickID(t, driver, rightEditorID)
	right.Editor.SetSelection(text.Range{Start: 0, End: right.Editor.RuneCount()})
	state.command("Sort In Locale Order Ascending")
	if left.Editor.String() != "left" || right.Editor.String() != "item1\nitem2\nitem10\n" || state.activePane != 1 {
		t.Fatalf("split locale sort = left:%q right:%q pane:%d", left.Editor.String(), right.Editor.String(), state.activePane)
	}
	if marks := state.bookmarkLines(right); len(marks) != 0 {
		t.Fatalf("locale sort bookmark = %v", marks)
	}
	state.command("Undo")
	if right.Editor.String() != "item10\nitem2\nitem1\n" || len(state.bookmarkLines(right)) != 0 {
		t.Fatalf("locale undo = %q marks=%v", right.Editor.String(), state.bookmarkLines(right))
	}
	state.command("Sort In Locale Order Descending")
	if right.Editor.String() != "item10\nitem2\nitem1\n" || !right.Editor.CanUndo() {
		t.Fatalf("already descending locale sort = %q", right.Editor.String())
	}
}

func TestRandomSortCommandPreservesLineMultiset(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo\nthree\nfour\nfive\n")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(len([]rune("one\ntwo\n")), false)
	state.toggleBookmark()
	beforeValue := doc.Editor.String()
	before := strings.Split(beforeValue, "\n")
	state.command("Randomize Line Order")
	after := strings.Split(doc.Editor.String(), "\n")
	slices.Sort(before)
	slices.Sort(after)
	if !slices.Equal(before, after) || state.notice == "" {
		t.Fatalf("random sort lines=%v notice=%q", after, state.notice)
	}
	if marks := state.bookmarkLines(doc); len(marks) != 0 {
		t.Fatalf("random sort did not follow source whole-buffer marker policy: %v", marks)
	}
	if doc.Editor.CanUndo() {
		state.command("Undo")
		if doc.Editor.String() != "one\ntwo\nthree\nfour\nfive\n" || len(state.bookmarkLines(doc)) != 0 {
			t.Fatalf("random undo = %q, marks=%v", doc.Editor.String(), state.bookmarkLines(doc))
		}
	}
}
