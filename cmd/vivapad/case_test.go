package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestConvertCaseMenuChangesSelectionAndUndoes(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("pre Ab猫c post"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(4, 8); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(11))
	if !state.caseMenuOpen {
		t.Fatal("Convert Case submenu did not open")
	}
	clickID(t, driver, caseScope.At(0))
	if got := state.space.Current().Editor.String(); got != "pre AB猫C post" || state.caseMenuOpen {
		t.Fatalf("uppercase menu = %q open=%v", got, state.caseMenuOpen)
	}
	if !state.space.Current().Editor.Undo() || state.space.Current().Editor.String() != "pre Ab猫c post" {
		t.Fatalf("uppercase undo = %q", state.space.Current().Editor.String())
	}
	clickID(t, driver, editorID)
	if err := driver.SelectText(4, 8); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(11))
	clickID(t, driver, caseScope.At(1))
	if got := state.space.Current().Editor.String(); got != "pre ab猫c post" {
		t.Fatalf("lowercase menu = %q", got)
	}
}

func TestInvertCaseMenuAndNoSelectionFeedback(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("aB猫"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(0, 3); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(11))
	clickID(t, driver, caseScope.At(6))
	if got := state.space.Current().Editor.String(); got != "Ab猫" {
		t.Fatalf("invert-case menu = %q", got)
	}
	state.space.Current().Editor.MoveTo(0, false)
	state.command("UPPERCASE")
	if state.space.Current().Editor.String() != "Ab猫" || !strings.Contains(state.notice, "Select text") {
		t.Fatalf("unselected conversion = %q notice=%q", state.space.Current().Editor.String(), state.notice)
	}
}

func TestConvertCaseSubmenuEscapeDoesNotEdit(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("Keep"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(11))
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.caseMenuOpen || state.space.Current().Editor.String() != "Keep" {
		t.Fatalf("Escape changed text or left case submenu open: %q open=%v", state.space.Current().Editor.String(), state.caseMenuOpen)
	}
}

func TestConvertCaseKeyboardShortcutsUseCurrentSelection(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("Ab猫"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(0, 3); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'u'); err != nil || !handled {
		t.Fatalf("uppercase shortcut handled=%v err=%v", handled, err)
	}
	if got := state.space.Current().Editor.String(); got != "AB猫" {
		t.Fatalf("uppercase shortcut = %q", got)
	}
	clickID(t, driver, editorID)
	if err := driver.SelectText(0, 3); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(primary(), 'u'); err != nil || !handled {
		t.Fatalf("lowercase shortcut handled=%v err=%v", handled, err)
	}
	if got := state.space.Current().Editor.String(); got != "ab猫" {
		t.Fatalf("lowercase shortcut = %q", got)
	}
}
