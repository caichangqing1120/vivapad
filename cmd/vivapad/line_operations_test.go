package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestLineOperationsMenuSortsSelectionAndUndoes(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("z\nb\na\noutside\n"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(0, 4); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	if !state.lineMenuOpen {
		t.Fatal("Line Operations submenu did not open")
	}
	clickID(t, driver, lineScope.At(0))
	if got := state.space.Current().Editor.String(); got != "b\nz\na\noutside\n" || state.lineMenuOpen {
		t.Fatalf("menu sorted text = %q open=%v", got, state.lineMenuOpen)
	}
	if !state.space.Current().Editor.Undo() || state.space.Current().Editor.String() != "z\nb\na\noutside\n" {
		t.Fatalf("menu sort undo = %q", state.space.Current().Editor.String())
	}
}

func TestLineOperationsMenuDeduplicatesOnlySelectedLines(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("b\nb\nc\nb\n"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(0, 4); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, lineScope.At(2))
	if got := state.space.Current().Editor.String(); got != "b\nc\nb\n" {
		t.Fatalf("selected dedup = %q", got)
	}
}

func TestLineOperationsSubmenuEscapeLeavesTextUntouched(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("z\na"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.lineMenuOpen || state.space.Current().Editor.String() != "z\na" {
		t.Fatalf("Escape changed lines or left submenu open: %+v", state)
	}
}

func TestLineOperationsCaseInsensitiveMenuKeepsEqualKeysStable(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("a\nB\nA\nb\n"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, lineScope.At(3))
	if got := state.space.Current().Editor.String(); got != "\na\nA\nB\nb" {
		t.Fatalf("case-insensitive ascending menu = %q", got)
	}
	state.space.Current().Editor.Undo()
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, lineScope.At(4))
	if got := state.space.Current().Editor.String(); got != "B\nb\na\nA\n" {
		t.Fatalf("case-insensitive descending menu = %q", got)
	}
}

func TestLineOperationsBlankAndConsecutiveCommands(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("b\nb\n \t\nc\nb\n"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, lineScope.At(7))
	if got := state.space.Current().Editor.String(); got != "b\n \t\nc\nb\n" {
		t.Fatalf("consecutive menu = %q", got)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, lineScope.At(6))
	if got := state.space.Current().Editor.String(); got != "b\nc\nb" {
		t.Fatalf("blank-line menu = %q", got)
	}
}
