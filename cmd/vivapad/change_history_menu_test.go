package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

// H06: the Search > Change History submenu keeps the original wording, dispatches
// the three commands, and stays honest about an unavailable history.

// The expected entries are written out here, independently of the table the
// implementation uses, so a renamed or reordered entry fails this test.
var wantChangeHistoryEntries = []struct{ label, command string }{
	{"Go to Next Change", "Next Change"},
	{"Go to Previous Change", "Previous Change"},
	{"Clear Change History", "Clear Change History"},
}

// submenuDocument opens a two-change fixture: lines 1 and 3 are modified so Next
// and Previous land on different lines.
func submenuDocument(t *testing.T) (*model, *app.Driver, *text.Editor) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "submenu.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	state, driver := startDriver(t)
	if err := state.space.Open(path); err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("A")
	doc.Editor.MoveTo(len([]rune("one\nAtwo\nthree\n")), false)
	doc.Editor.Insert("B")
	doc.Editor.MoveTo(0, false)
	return state, driver, doc.Editor
}

func submenuHits(t *testing.T, driver *app.Driver) map[string]gfx.Hit {
	t.Helper()
	if err := driver.Pump(); err != nil {
		t.Fatalf("pump: %v", err)
	}
	hits := map[string]gfx.Hit{}
	for _, hit := range driver.Snapshot().Hits {
		for index := range changeHistoryItems {
			if hit.ID == changeHistoryScope.At(index) {
				hits[hit.Label] = hit
			}
		}
	}
	return hits
}

func TestChangeHistorySubmenuRendersOriginalEntries(t *testing.T) {
	state, driver, _ := submenuDocument(t)
	state.command("Change History")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatalf("pump: %v", err)
	}
	hits := submenuHits(t, driver)
	if len(hits) != len(wantChangeHistoryEntries) {
		t.Fatalf("painted %d submenu entries (%v), want %d", len(hits), hits, len(wantChangeHistoryEntries))
	}
	for _, want := range wantChangeHistoryEntries {
		hit, ok := hits[want.label]
		if !ok {
			t.Fatalf("the submenu does not paint %q; painted %v", want.label, hits)
		}
		if hit.Disabled {
			t.Fatalf("%q is disabled while the history is available", want.label)
		}
	}
	for index, item := range changeHistoryItems {
		if index >= len(wantChangeHistoryEntries) || item.label != wantChangeHistoryEntries[index].label || item.command != wantChangeHistoryEntries[index].command {
			t.Fatalf("entry %d = %+v, want %+v", index, item, wantChangeHistoryEntries[index])
		}
	}
}

func TestChangeHistorySubmenuClicksDispatchEachEntry(t *testing.T) {
	state, driver, editor := submenuDocument(t)
	lineOf := func() int {
		return strings.Count(editor.String()[:editor.ByteOffset(editor.Caret())], "\n")
	}
	// Next goes forward from the caret (line 0 -> line 1), Previous backwards from
	// the document start wraps to the last change (line 3).
	for _, want := range []struct {
		index       int
		line, back  int
		description string
	}{
		{0, 1, 0, "next change"},
		{1, 3, 0, "previous change"},
	} {
		editor.MoveTo(0, false)
		state.command("Change History")
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatalf("pump: %v", err)
		}
		clickID(t, driver, changeHistoryScope.At(want.index))
		if got := lineOf(); got != want.line {
			t.Fatalf("%s landed on line %d, want %d", want.description, got, want.line)
		}
		if state.changeHistoryMenuOpen || state.menuOpen >= 0 {
			t.Fatalf("%s left the menu open: submenu=%v menu=%d", want.description, state.changeHistoryMenuOpen, state.menuOpen)
		}
	}
	// Clear through the same path resets the history.
	state.command("Change History")
	if err := driver.Pump(); err != nil {
		t.Fatalf("pump: %v", err)
	}
	clickID(t, driver, changeHistoryScope.At(2))
	if state.notice != "Change history cleared" {
		t.Fatalf("clear notice = %q", state.notice)
	}
	if editor.CanUndo() {
		t.Fatal("clear through the submenu kept undo units")
	}
}

func TestChangeHistorySubmenuDisabledHitsAreInert(t *testing.T) {
	state, driver, editor := submenuDocument(t)
	editor = text.NewEditor("replaced")
	state.space.Current().Editor = editor
	state.command("Change History")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatalf("pump: %v", err)
	}
	hits := submenuHits(t, driver)
	for _, label := range []string{"Go to Next Change", "Go to Previous Change"} {
		if hit, ok := hits[label]; !ok || !hit.Disabled {
			t.Fatalf("%q is not painted disabled: %+v", label, hit)
		}
	}
	if hit, ok := hits["Clear Change History"]; !ok || hit.Disabled {
		t.Fatalf("Clear must stay available: %+v", hit)
	}
	before := editor.Caret()
	clickID(t, driver, changeHistoryScope.At(0))
	if editor.Caret() != before {
		t.Fatal("a disabled entry still navigated")
	}
	// A dispatch would have moved the caret or cleared the history; both are intact.
	if editor.String() != "replaced" {
		t.Fatalf("a disabled entry changed the text to %q", editor.String())
	}
	if !strings.Contains(state.notice, "change-history source") {
		t.Fatalf("notice = %q, want the unavailable reason", state.notice)
	}
	if strings.Contains(state.notice, "No changes") {
		t.Fatalf("unavailable history reported %q", state.notice)
	}
}

func TestChangeHistorySubmenuEscapeClosesOnlyIt(t *testing.T) {
	state, driver, _ := submenuDocument(t)
	state.command("Change History")
	if err := driver.Pump(); err != nil {
		t.Fatalf("pump: %v", err)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatalf("escape: %v", err)
	}
	if state.changeHistoryMenuOpen {
		t.Fatal("Escape did not dismiss the Change History submenu")
	}
	if state.menuOpen != 2 {
		t.Fatalf("Escape closed the Search menu too: menuOpen=%d", state.menuOpen)
	}
}

func TestChangeHistoryClearRefusalDismissesMenus(t *testing.T) {
	state, driver, _ := submenuDocument(t)
	editor := text.NewEditor("replaced")
	state.space.Current().Editor = editor
	state.command("Change History")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatalf("pump: %v", err)
	}
	clickID(t, driver, changeHistoryScope.At(2))
	if state.changeHistoryMenuOpen || state.menuOpen >= 0 {
		t.Fatalf("a refused Clear left the menus open: submenu=%v menu=%d", state.changeHistoryMenuOpen, state.menuOpen)
	}
	if !strings.Contains(state.notice, "change-history source") {
		t.Fatalf("refusal notice = %q", state.notice)
	}
	if editor.String() != "replaced" {
		t.Fatalf("a refused Clear changed the text to %q", editor.String())
	}
}

func TestChangeHistorySubmenuReachableAtCompactSize(t *testing.T) {
	state, driver, _ := submenuDocument(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatalf("resize: %v", err)
	}
	index := slices.Index(menuCommands[2], "Change History")
	if index < 0 {
		t.Fatal("the Change History entry is missing from the Search menu")
	}
	clickID(t, driver, menuScope.At(2))
	if err := driver.Pump(); err != nil {
		t.Fatalf("pump: %v", err)
	}
	// Scroll with real wheel events over the Search panel until the row is visible.
	panel := gfx.Rect{}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == searchCommandID(t, "Find...") {
			panel = hit.Bounds
		}
	}
	if panel.Width == 0 {
		t.Fatal("the Search panel is not painted")
	}
	center := gfx.Point{X: panel.X + panel.Width/2, Y: panel.Y + panel.Height/2}
	for step := 0; step < len(menuCommands[2]) && !state.entryVisible("Change History"); step++ {
		if err := driver.Scroll(gfx.ScrollEvent{X: center.X, Y: center.Y, DeltaY: 60}); err != nil {
			t.Fatalf("scroll: %v", err)
		}
		if err := driver.Pump(); err != nil {
			t.Fatalf("pump: %v", err)
		}
	}
	if !state.entryVisible("Change History") {
		t.Fatalf("scrolling never revealed the entry (offset %d, visible %d)", state.searchMenuOffset, state.searchMenuVisible)
	}
	var target gfx.Hit
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(index) {
			target = hit
		}
	}
	if target.Label != "Change History" {
		t.Fatalf("the item ID %v labels %q, want Change History", menuItemScope.At(index), target.Label)
	}
	if err := driver.Click(target.Bounds.X+target.Bounds.Width/2, target.Bounds.Y+target.Bounds.Height/2); err != nil {
		t.Fatalf("click: %v", err)
	}
	if !state.changeHistoryMenuOpen || state.menuOpen != 2 {
		t.Fatalf("clicking the entry did not open the submenu: menu=%d open=%v", state.menuOpen, state.changeHistoryMenuOpen)
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact Change History submenu layout: %+v", problems)
	}
}

// entryVisible reports whether the Search menu currently shows the command's row.
func (state *model) entryVisible(command string) bool {
	index := slices.Index(menuCommands[2], command)
	return index >= state.searchMenuOffset && index < state.searchMenuOffset+state.searchMenuVisible
}

func TestChangeHistoryClearRefusalRequestsFocus(t *testing.T) {
	state, _, _ := submenuDocument(t)
	state.space.Current().Editor = text.NewEditor("replaced")
	state.command("Clear Change History")
	if !state.focusViewRequested {
		t.Fatal("a refused Clear did not hand focus back to the editor")
	}
}

func TestChangeHistorySubmenuClickThenTypingLandsInEditor(t *testing.T) {
	state, driver, editor := submenuDocument(t)
	state.command("Change History")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatalf("pump: %v", err)
	}
	clickID(t, driver, changeHistoryScope.At(0))
	before := editor.String()
	caret := editor.Caret()
	if err := driver.Type("Q"); err != nil {
		t.Fatalf("type: %v", err)
	}
	if editor.String() == before || editor.Caret() <= caret {
		t.Fatalf("typing after the submenu command did not reach the editor: %q caret %d", editor.String(), editor.Caret())
	}
}

// H06: in a split view the command acts on, and returns focus to, the active pane.
func TestChangeHistorySubmenuActsOnActiveRightPane(t *testing.T) {
	path := filepath.Join(t.TempDir(), "split.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left stays intact")
	if err := state.space.Open(path); err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	right := state.space.Current()
	state.showInActivePane(right)
	state.command("Split View")
	if !state.splitEnabled || state.paneDocument(0) != left || state.paneDocument(1) != right {
		t.Fatal("fixture did not create two distinct split documents")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	if state.activePane != 1 || state.space.Current() != right {
		t.Fatal("pointer click did not activate the right pane")
	}
	right.Editor.MoveTo(len([]rune("one\n")), false)
	right.Editor.Insert("B")
	right.Editor.MoveTo(0, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatalf("pump: %v", err)
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[2], "Change History")))
	clickID(t, driver, changeHistoryScope.At(0))
	if state.space.Current() != right || state.activePane != 1 || driver.Snapshot().Focused != rightEditorID {
		t.Fatal("the command did not return focus to the right editor")
	}
	line := strings.Count(right.Editor.String()[:right.Editor.ByteOffset(right.Editor.Caret())], "\n")
	if line != 1 {
		t.Fatalf("right pane caret on line %d, want the changed line 1", line)
	}
	if err := driver.Type("Z"); err != nil {
		t.Fatalf("type: %v", err)
	}
	if right.Editor.String() != "one\nZBtwo\nthree\n" || left.Editor.String() != "left stays intact" {
		t.Fatalf("typing after the command edited the wrong target: right=%q left=%q", right.Editor.String(), left.Editor.String())
	}
}
