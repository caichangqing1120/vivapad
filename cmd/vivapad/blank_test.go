package main

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestBlankSubmenuTrimsSelectedLineAndUndoes(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("  a \t\n b  ")
	doc.Editor.SetSelection(text.Range{Start: 3, End: 4})
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(12))
	clickID(t, driver, blankScope.At(2))
	if got := doc.Editor.String(); got != "a\n b  " {
		t.Fatalf("selected trim = %q", got)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "  a \t\n b  " {
		t.Fatalf("trim menu undo = %q", doc.Editor.String())
	}
}

func TestBlankSubmenuFitsCompactWindowAndJoinsLines(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("a\nb")
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(12))
	visible := 0
	for _, hit := range driver.Snapshot().Hits {
		for index := range blankCommands {
			if hit.ID == blankScope.At(index) {
				visible++
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("blank row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
	}
	if visible != len(blankCommands) {
		t.Fatalf("blank menu rows = %d, want %d", visible, len(blankCommands))
	}
	clickID(t, driver, blankScope.At(3))
	if doc.Editor.String() != "a b" || state.blankMenuOpen || state.menuOpen != -1 {
		t.Fatalf("EOL to Space menu = %q, open=%v parent=%d", doc.Editor.String(), state.blankMenuOpen, state.menuOpen)
	}
}

func TestBlankTabConversionKeepsBookmarkIdentity(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\tb\n    c")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	state.configureCode(doc)
	state.command("TAB to Space")
	if doc.Editor.String() != "a   b\n    c" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("TAB to Space markers = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	state.command("Space to TAB (Leading)")
	if doc.Editor.String() != "a   b\n\tc" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("leading Space to TAB markers = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	if !doc.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("tab conversion undo markers = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestBlankOperationHonorsConfiguredTabWidth(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	state.tabWidth = 8
	state.configureCode(doc)
	doc.Editor.Insert("a\tb")
	state.command("TAB to Space")
	if doc.Editor.String() != "a       b" || doc.LineEnding() != document.LF {
		t.Fatalf("configured tab width = %q", doc.Editor.String())
	}
}

func TestAllBlankMenuCommandsEditAndUndoThroughDriver(t *testing.T) {
	for _, test := range []struct {
		index        int
		before, want string
	}{
		{0, " a \n", " a\n"},
		{1, " a \n", "a \n"},
		{2, " a \n", "a\n"},
		{3, "a\nb", "a b"},
		{4, " a \n b ", "a b"},
		{5, "a\tb", "a   b"},
		{6, "a   b", "a\tb"},
		{7, "    b", "\tb"},
	} {
		t.Run(blankCommands[test.index], func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert(test.before)
			clickID(t, driver, menuScope.At(1))
			clickID(t, driver, menuItemScope.At(12))
			clickID(t, driver, blankScope.At(test.index))
			if got := doc.Editor.String(); got != test.want {
				t.Fatalf("menu command = %q, want %q", got, test.want)
			}
			if !doc.Editor.Undo() || doc.Editor.String() != test.before {
				t.Fatalf("menu command did not undo: %q", doc.Editor.String())
			}
		})
	}
}
