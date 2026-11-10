package main

import (
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestFindPreviousButtonMenuAndShortcutWrap(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("one two one"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.MoveTo(0, false)
	state.query.Insert("one")
	clickID(t, driver, searchID)
	clickID(t, driver, findPreviousID)
	if got := doc.Editor.Selection(); got != (text.Range{Start: 8, End: 11}) {
		t.Fatalf("previous button did not wrap: %+v", got)
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(2))
	if got := doc.Editor.Selection(); got != (text.Range{Start: 0, End: 3}) {
		t.Fatalf("previous menu did not move back: %+v", got)
	}
	if err := driver.Key(ui.KeyF3, ui.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.Selection(); got != (text.Range{Start: 8, End: 11}) {
		t.Fatalf("Shift+F3 did not wrap: %+v", got)
	}
}

func TestEditSelectAllIndentAndOutdentMenu(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("a\nb"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(5))
	if got := doc.Editor.Selection(); got != (text.Range{Start: 0, End: 3}) {
		t.Fatalf("Select All menu selection = %+v", got)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(8))
	if doc.Editor.String() != "\ta\n\tb" {
		t.Fatalf("Indent Lines menu = %q", doc.Editor.String())
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(9))
	if doc.Editor.String() != "a\nb" {
		t.Fatalf("Unindent Lines menu = %q", doc.Editor.String())
	}
}

func TestDescendingLexicographicSortThroughLineMenu(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("a\nb\nc"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, lineScope.At(1))
	if got := state.space.Current().Editor.String(); got != "c\nb\na" {
		t.Fatalf("descending sort menu = %q", got)
	}
}
