package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestChangeHistoryKeyboardWrapsAndPreservesDocumentUntilActivation(t *testing.T) {
	state, driver, editor := submenuDocument(t)
	before, selection := editor.String(), editor.Selection()
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Change History"))
	for _, step := range []struct {
		key   gfx.Key
		index int
	}{
		{gfx.KeyDown, 0}, {gfx.KeyDown, 1}, {gfx.KeyDown, 2}, {gfx.KeyDown, 0},
		{gfx.KeyUp, 2}, {gfx.KeyHome, 0}, {gfx.KeyEnd, 2},
	} {
		if err := driver.Key(step.key, 0); err != nil {
			t.Fatal(err)
		}
		if driver.Snapshot().Focused != changeHistoryScope.At(step.index) {
			t.Fatalf("key %v focused %v, want row %d", step.key, driver.Snapshot().Focused, step.index)
		}
		if !state.changeHistoryMenuOpen || editor.String() != before || editor.Selection() != selection {
			t.Fatal("menu navigation executed an action or changed the document")
		}
	}
}

func TestChangeHistoryKeyboardSkipsUnavailableNavigation(t *testing.T) {
	state, driver, _ := submenuDocument(t)
	editor := text.NewEditor("replacement")
	state.space.Current().Editor = editor
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Change History"))
	for _, key := range []gfx.Key{gfx.KeyDown, gfx.KeyUp, gfx.KeyHome, gfx.KeyEnd} {
		if err := driver.Key(key, 0); err != nil {
			t.Fatal(err)
		}
		if driver.Snapshot().Focused != changeHistoryScope.At(2) {
			t.Fatal("keyboard focused a disabled history navigation command")
		}
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "replacement" || state.menuOpen >= 0 || state.changeHistoryMenuOpen || !strings.Contains(state.notice, "change-history source") {
		t.Fatalf("keyboard Clear refusal changed data or lost its reason: %q / %q", editor.String(), state.notice)
	}
}

func TestChangeHistoryKeyboardCloseRestoresParentAndCanReopen(t *testing.T) {
	for _, key := range []gfx.Key{gfx.KeyLeft, gfx.KeyEscape} {
		state, driver, _ := submenuDocument(t)
		state.query.Insert("one")
		state.command("Find All in Current Document")
		if !state.resultsOpen {
			t.Fatal("fixture did not open search results")
		}
		clickID(t, driver, menuScope.At(2))
		parent := searchCommandID(t, "Change History")
		clickID(t, driver, parent)
		if err := driver.Key(gfx.KeyDown, 0); err != nil {
			t.Fatal(err)
		}
		if err := driver.Key(key, 0); err != nil {
			t.Fatal(err)
		}
		if state.menuOpen != 2 || state.changeHistoryMenuOpen || driver.Snapshot().Focused != parent || !state.resultsOpen {
			t.Fatalf("key %v did not return focus to the Search parent", key)
		}
		if err := driver.Key(gfx.KeyRight, 0); err != nil {
			t.Fatal(err)
		}
		if !state.changeHistoryMenuOpen || driver.Snapshot().Focused != changeHistoryScope.At(0) {
			t.Fatal("Right did not reopen and focus the first Change History action")
		}
	}
}

func TestChangeHistoryKeyboardTakesOverPointerHighlight(t *testing.T) {
	_, driver, _ := submenuDocument(t)
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Change History"))
	second := syncScrollHit(t, driver, changeHistoryScope.At(1))
	if err := driver.Move(second.X+20, second.Y+10); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyUp, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != changeHistoryScope.At(0) {
		t.Errorf("Up from the hovered second row focused %v, want the first", driver.Snapshot().Focused)
	}
	first := syncScrollHit(t, driver, changeHistoryScope.At(0))
	highlights := 0
	for _, primitive := range driver.Snapshot().Primitives {
		if primitive.Layer == 3 && primitive.Color == classicPalette.hover {
			highlights++
			if primitive.Bounds != first {
				t.Errorf("keyboard highlight remains on a different row: %+v", primitive.Bounds)
			}
		}
	}
	if highlights != 1 {
		t.Errorf("highlighted submenu rows = %d, want 1", highlights)
	}
}

func TestChangeHistoryKeyboardDoesNotCaptureTabFocusOutsideSubmenu(t *testing.T) {
	for _, test := range []struct {
		name string
		key  gfx.Key
	}{{"Enter", gfx.KeyEnter}, {"Up", gfx.KeyUp}, {"Down", gfx.KeyDown}} {
		t.Run(test.name, func(t *testing.T) {
			state, driver, editor := submenuDocument(t)
			before, selection := editor.String(), editor.Selection()
			clickID(t, driver, menuScope.At(2))
			clickID(t, driver, searchCommandID(t, "Change History"))
			if err := driver.Key(gfx.KeyDown, 0); err != nil {
				t.Fatal(err)
			}
			root := searchCommandID(t, "Find...")
			for count := 0; driver.Snapshot().Focused != root && count < 64; count++ {
				if err := driver.Key(gfx.KeyTab, gfx.ModifierShift); err != nil {
					t.Fatal(err)
				}
			}
			if driver.Snapshot().Focused != root {
				t.Fatal("Shift+Tab did not reach the other Search root item")
			}
			if err := driver.Key(test.key, 0); err != nil {
				t.Fatal(err)
			}
			if driver.Snapshot().Focused.Scope() == changeHistoryScope {
				t.Fatalf("key %v was captured by the submenu after focus left it", test.key)
			}
			if test.key == gfx.KeyEnter && (!state.searchOpen || state.menuOpen >= 0 || state.changeHistoryMenuOpen) {
				t.Fatal("Enter did not activate the focused Find root command")
			}
			if editor.String() != before || editor.Selection() != selection {
				t.Fatal("keys outside the submenu changed the source document")
			}
		})
	}
}
