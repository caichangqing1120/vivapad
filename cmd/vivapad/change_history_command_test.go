package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func openChangeHistoryFixture(t *testing.T, value string) (*model, *app.Driver) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "changes.txt")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	return state, driver
}

func TestChangeHistorySearchSubmenuExposesAndRunsCommands(t *testing.T) {
	state, driver := openChangeHistoryFixture(t, "one\ntwo\nthree\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	doc.Editor.MoveTo(0, false)
	state.command("Change History")
	if !state.changeHistoryMenuOpen || state.menuOpen != 2 {
		t.Fatal("Change History did not open the Search submenu")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for index, label := range []string{"Go to Next Change", "Go to Previous Change", "Clear Change History"} {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == changeHistoryScope.At(index) && hit.Label == label && !hit.Disabled {
				found = true
			}
		}
		if !found {
			t.Fatalf("submenu entry %q is missing or disabled", label)
		}
	}
	clickID(t, driver, changeHistoryScope.At(0))
	if doc.Editor.Caret() != 4 || state.changeHistoryMenuOpen || state.menuOpen >= 0 {
		t.Fatalf("submenu did not navigate or close: caret=%d menu=%v", doc.Editor.Caret(), state.changeHistoryMenuOpen)
	}
}

func TestChangeHistorySearchMenuReachableAtCompactSize(t *testing.T) {
	state, driver := openChangeHistoryFixture(t, "one\ntwo\n")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	index := slices.Index(menuCommands[2][:], "Change History")
	if index < 0 {
		t.Fatal("Search menu has no Change History entry")
	}
	clickID(t, driver, menuScope.At(2))
	for step := 0; step < len(menuCommands[2]); step++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		visible := false
		panel := gfx.Rect{}
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == menuItemScope.At(index) {
				visible = true
			}
			if hit.ID == searchMenuPanelID {
				panel = hit.Bounds
			}
		}
		if visible {
			break
		}
		if panel.Width == 0 {
			t.Fatal("Search menu panel is missing")
		}
		if err := driver.Scroll(gfx.ScrollEvent{X: panel.X + panel.Width/2, Y: panel.Y + panel.Height/2, DeltaY: 60}); err != nil {
			t.Fatal(err)
		}
	}
	clickID(t, driver, menuItemScope.At(index))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.changeHistoryMenuOpen {
		t.Fatal("compact Search menu did not open Change History")
	}
	for subindex := range changeHistoryItems {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == changeHistoryScope.At(subindex) && hit.Bounds.X >= 0 && hit.Bounds.Y >= 0 && hit.Bounds.Right() <= 640 && hit.Bounds.Bottom() <= 400 {
				found = true
			}
		}
		if !found {
			t.Fatalf("compact submenu entry %d is outside the window", subindex)
		}
	}
}

func TestChangeHistorySubmenuKeyboardNavigationAndUnavailableState(t *testing.T) {
	state, driver := openChangeHistoryFixture(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	parent := menuItemScope.At(slices.Index(menuCommands[2][:], "Change History"))
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, parent)
	for _, item := range []struct {
		key  gfx.Key
		want int
	}{{gfx.KeyDown, 0}, {gfx.KeyDown, 1}, {gfx.KeyDown, 2}, {gfx.KeyDown, 0}, {gfx.KeyUp, 2}} {
		if err := driver.Key(item.key, 0); err != nil {
			t.Fatal(err)
		}
		if driver.Snapshot().Focused != changeHistoryScope.At(item.want) {
			t.Fatalf("key %v focused %v, want %d", item.key, driver.Snapshot().Focused, item.want)
		}
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.changeHistoryMenuOpen || state.menuOpen != 2 || driver.Snapshot().Focused != parent {
		t.Fatal("Escape did not return to the Search parent")
	}
	if err := driver.Key(gfx.KeyRight, 0); err != nil {
		t.Fatal(err)
	}
	if !state.changeHistoryMenuOpen || driver.Snapshot().Focused != changeHistoryScope.At(0) {
		t.Fatal("Right did not reopen the submenu")
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Caret() != 4 || state.menuOpen >= 0 {
		t.Fatal("Enter did not navigate and close the menu")
	}

	doc.Editor = text.NewEditor("replacement")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, parent)
	if err := driver.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != changeHistoryScope.At(2) {
		t.Fatal("unavailable history focused disabled navigation")
	}
}

func TestChangeHistoryCommandsNavigateBlocksAndWrap(t *testing.T) {
	state, _ := openChangeHistoryFixture(t, "one\ntwo\nthree\nfour\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	doc.Editor.MoveTo(len([]rune("oneX\ntwo\nthree\n")), false)
	doc.Editor.Insert("Y")
	doc.Editor.MoveTo(0, false)
	line := func() int {
		return strings.Count(doc.Editor.String()[:doc.Editor.ByteOffset(doc.Editor.Caret())], "\n")
	}
	for _, want := range []int{1, 3, 1} {
		state.command("Next Change")
		if got := line(); got != want {
			t.Fatalf("Next Change line = %d, want %d; notice = %q", got, want, state.notice)
		}
	}
	state.command("Previous Change")
	if got := line(); got != 3 {
		t.Fatalf("Previous Change line = %d, want 3", got)
	}
}

func TestChangeHistoryCommandsPreserveUnchangedSelectionAndDirtyClear(t *testing.T) {
	state, _ := openChangeHistoryFixture(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.SetSelection(text.Range{Start: 1, End: 3})
	before := doc.Editor.Selection()
	state.command("Next Change")
	if doc.Editor.Selection() != before || state.notice != "No changes to navigate" {
		t.Fatalf("unchanged selection moved or notice wrong: %v, %q", doc.Editor.Selection(), state.notice)
	}
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	value := doc.Editor.String()
	state.command("Clear Change History")
	if doc.Editor.String() != value || !doc.Dirty() || doc.Editor.CanUndo() || state.notice != "Change history cleared" {
		t.Fatalf("clear changed document state: text=%q dirty=%v notice=%q", doc.Editor.String(), doc.Dirty(), state.notice)
	}
	state.command("Next Change")
	if state.notice != "No changes to navigate" {
		t.Fatalf("clear retained change markers: %q", state.notice)
	}
}

func TestClearChangeHistoryCommandRefusesLiveComposition(t *testing.T) {
	state, _ := openChangeHistoryFixture(t, "abc\n")
	doc := state.space.Current()
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("tagged composition unavailable")
	}
	doc.Editor.MoveTo(1, false)
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "か"}) {
		t.Fatal("cannot start composition")
	}
	before := doc.Editor.String()
	state.command("Clear Change History")
	if !strings.Contains(state.notice, "composition") || !doc.Editor.CompositionActive() || doc.Editor.String() != before {
		t.Fatalf("refused Clear changed composition: %q, active=%v, notice=%q", doc.Editor.String(), doc.Editor.CompositionActive(), state.notice)
	}
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel}) || doc.Dirty() {
		t.Fatal("refused Clear prevented cancellation or changed the save boundary")
	}
}
