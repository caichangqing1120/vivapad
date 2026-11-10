package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestLineExtraMenuFitsCompactAndJoinsSelectedLines(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("a\nb\nc\n"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(0, 4); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	for index := 8; index <= 12; index++ {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == lineScope.At(index) {
				found = true
				if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("line command %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("line command %d missing", index)
		}
	}
	clickID(t, driver, lineScope.At(8))
	if got := state.space.Current().Editor.String(); got != "a b\nc\n" {
		t.Fatalf("Join Lines menu = %q", got)
	}
	state.command("Undo")
	state.space.Current().Editor.SetSelection(text.Range{Start: 0, End: 4})
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, lineScope.At(9))
	if got := state.space.Current().Editor.String(); got != "b\na\nc\n" {
		t.Fatalf("Reverse Line Order menu = %q", got)
	}
}

func TestLineExtraBookmarkMovesAndReadOnlyBlocks(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb\nc\n")
	doc.Editor.MoveTo(0, false)
	state.command("Toggle Bookmark")
	state.command("Reverse Line Order")
	if doc.Editor.String() != "\nc\nb\na" || len(state.bookmarkLines(doc)) != 0 {
		t.Fatalf("reverse bookmark = %q, marks=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	state.command("Undo")
	doc.Editor.MoveTo(0, false)
	state.command("Toggle Bookmark")
	doc.Editor.MoveTo(2, false)
	state.command("Toggle Bookmark")
	state.command("Insert Blank Line Above Current")
	if doc.Editor.String() != "a\n\nb\nc\n" || len(state.bookmarkLines(doc)) != 2 || state.bookmarkLines(doc)[0] != 0 || state.bookmarkLines(doc)[1] != 2 {
		t.Fatalf("blank-above bookmark = %q, marks=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	state.command("Read-Only on Current Document")
	before := doc.Editor.String()
	for _, command := range []string{"Join Lines", "Reverse Line Order", "Insert Blank Line Above Current", "Insert Blank Line Below Current"} {
		state.command(command)
		if doc.Editor.String() != before || !strings.Contains(state.notice, "Read-only") {
			t.Fatalf("read-only %q changed text or notice: %q, %q", command, doc.Editor.String(), state.notice)
		}
	}
}

func TestLineExtraBlankBelowOnlyChangesActiveSplitDocument(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	state.command("Insert Blank Line Below Current")
	if left.Editor.String() != "left" || right.Editor.String() != "right\n" || right.Editor.Caret() != 6 {
		t.Fatalf("split blank-below = left:%q right:%q caret:%d", left.Editor.String(), right.Editor.String(), right.Editor.Caret())
	}
}
