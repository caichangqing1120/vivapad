package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestFindByteRangeDialogDefaultUnicodeAndRepeatedFind(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("A猫B界")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(20))
	if !state.byteRangeOpen || state.byteRangeMode != 0 || state.byteRangeUp || state.byteRangeWrap {
		t.Fatal("Find characters in range did not open with original defaults")
	}
	clickID(t, driver, byteRangeFindID)
	if doc.Editor.Selection() != (text.Range{Start: 1, End: 2}) || !state.byteRangeOpen || doc.Editor.CanUndo() {
		t.Fatalf("first byte-range match = %+v, open=%v", doc.Editor.Selection(), state.byteRangeOpen)
	}
	clickID(t, driver, byteRangeFindID)
	if doc.Editor.Selection() != (text.Range{Start: 3, End: 4}) || doc.Editor.String() != "A猫B界" {
		t.Fatalf("second byte-range match = %+v", doc.Editor.Selection())
	}
	clickID(t, driver, byteRangeCloseID)
	if state.byteRangeOpen {
		t.Fatal("Close did not dismiss the byte-range dialog")
	}
}

func TestFindByteRangeCustomValidationWrapAndSplitReadOnly(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	state.command("Split View")
	state.activatePane(1)
	right := state.space.NewDocument()
	state.showInActivePane(right)
	right.Editor.Insert("a0b2")
	right.Editor.ClearHistory()
	right.Editor.MoveTo(0, false)
	state.readOnly[right] = true
	state.command("Find characters in range...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, byteRangeCustomID)
	state.byteRangeStart = text.NewEditor("57")
	state.byteRangeEnd = text.NewEditor("48")
	clickID(t, driver, byteRangeFindID)
	if state.byteRangeError == "" || right.Editor.Selection() != (text.Range{Start: 0, End: 0}) {
		t.Fatal("invalid custom range moved selection")
	}
	state.byteRangeStart = text.NewEditor("48")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, byteRangeFindID)
	if right.Editor.Selection() != (text.Range{Start: 1, End: 2}) || left.Editor.String() != "left" || right.Editor.CanUndo() {
		t.Fatal("byte-range search did not stay in active split pane")
	}
	clickID(t, driver, byteRangeUpID)
	clickID(t, driver, byteRangeWrapID)
	clickID(t, driver, byteRangeFindID)
	if right.Editor.Selection() != (text.Range{Start: 1, End: 2}) || state.byteRangeError != "" {
		t.Fatalf("backward wrap = %+v error=%q", right.Editor.Selection(), state.byteRangeError)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.byteRangeOpen {
		t.Fatalf("Escape did not close byte-range dialog: %v", err)
	}
}

func TestFindByteRangeCompactSearchMenuScroll(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	if state.searchMenuVisible >= len(menuCommands[2]) || state.searchMenuVisible < 8 {
		t.Fatalf("compact Search visible rows = %d", state.searchMenuVisible)
	}
	clickID(t, driver, searchMenuNextID)
	if state.searchMenuOffset != 1 {
		t.Fatalf("Search scroll offset = %d", state.searchMenuOffset)
	}
	clickID(t, driver, menuItemScope.At(20))
	if !state.byteRangeOpen {
		t.Fatal("last Search command was not reachable")
	}
}

func TestFindByteRangeWrapInvalidBoundsAndKeyboard(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("A猫B2")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.command("Find characters in range...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, byteRangeCustomID)
	clickID(t, driver, byteRangeStartID)
	if _, err := driver.ShortcutChord(primary(), 'a'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("50"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, byteRangeEndID)
	if _, err := driver.ShortcutChord(primary(), 'a'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("50"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, byteRangeUpID)
	clickID(t, driver, byteRangeFindID)
	if state.byteRangeError != "No character in range" || doc.Editor.Selection() != (text.Range{Start: 0, End: 0}) {
		t.Fatal("upward search wrapped while Wrap was off")
	}
	clickID(t, driver, byteRangeWrapID)
	clickID(t, driver, byteRangeFindID)
	if state.byteRangeError != "" || doc.Editor.Selection() != (text.Range{Start: 3, End: 4}) || doc.Editor.CanUndo() {
		t.Fatalf("upward wrap = %+v error=%q", doc.Editor.Selection(), state.byteRangeError)
	}
	state.byteRangeEnd = text.NewEditor("256")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, byteRangeFindID)
	if !state.byteRangeOpen || state.byteRangeError == "" || doc.Editor.Selection() != (text.Range{Start: 3, End: 4}) {
		t.Fatal("invalid bound changed selection or closed dialog")
	}
	state.byteRangeEnd = text.NewEditor("")
	state.findByteRange()
	if state.byteRangeError == "" || doc.Editor.Selection() != (text.Range{Start: 3, End: 4}) {
		t.Fatal("empty bound was accepted")
	}
	state.byteRangeEnd = text.NewEditor("50")
	state.byteRangeUp = false
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, byteRangeEndID)
	if err := driver.Key(gfx.KeyEnter, 0); err != nil || state.byteRangeError != "" || doc.Editor.Selection() != (text.Range{Start: 3, End: 4}) {
		t.Fatalf("Enter find = %+v error=%q, %v", doc.Editor.Selection(), state.byteRangeError, err)
	}
}

func TestFindByteRangeAllowsEditorAndWindowNavigation(t *testing.T) {
	state, driver := startDriver(t)
	state.command("Find characters in range...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, newID)
	if len(state.space.Documents) != 2 || !state.byteRangeOpen {
		t.Fatal("modeless byte-range dialog blocked New or closed unexpectedly")
	}
	clickID(t, driver, editorID)
	if err := driver.Type("A猫"); err != nil || state.space.Current().Editor.String() != "A猫" {
		t.Fatalf("modeless editor typing = %q, %v", state.space.Current().Editor.String(), err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil || state.space.Current().Editor.String() != "A猫\n" {
		t.Fatalf("modeless editor Enter = %q, %v", state.space.Current().Editor.String(), err)
	}
	state.space.Current().Editor.ClearHistory()
	state.space.Current().Editor.MoveTo(0, false)
	clickID(t, driver, byteRangeFindID)
	if state.space.Current().Editor.Selection() != (text.Range{Start: 1, End: 2}) {
		t.Fatalf("Find did not follow active document: %+v", state.space.Current().Editor.Selection())
	}
	closed := false
	state.space.Current().Editor.ClearHistory()
	state.space.Current().Editor.MoveTo(0, false)
	state.space.Current().Editor.ClearHistory()
	if err := state.space.Current().Save(filepath.Join(t.TempDir(), "new.txt")); err != nil {
		t.Fatal(err)
	}
	newConfig(state).OnCloseRequest(func() { closed = true })
	if !closed {
		t.Fatal("modeless byte-range dialog blocked a clean window close")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.byteRangeOpen {
		t.Fatalf("Escape failed to close dialog: %v", err)
	}
}

func TestFindByteRangeContinuationByteSelectsContainingRune(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("A猫B")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.query.Insert("unchanged query")
	state.command("Find characters in range...")
	state.byteRangeMode = 2
	state.byteRangeStart = text.NewEditor("140")
	state.byteRangeEnd = text.NewEditor("140")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, byteRangeFindID)
	if doc.Editor.Selection() != (text.Range{Start: 1, End: 2}) || doc.Editor.CanUndo() || state.query.String() != "unchanged query" || doc.Editor.String() != "A猫B" {
		t.Fatalf("continuation byte selection=%+v query=%q", doc.Editor.Selection(), state.query.String())
	}
}

func TestCaptureFindByteRangeWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_BYTE_RANGE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_BYTE_RANGE_SHOTS to capture byte-range dialog")
	}
	state := newModel()
	state.space.Current().Editor.Insert("A猫B界")
	state.command("Find characters in range...")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if size.name == "compact" {
			clickID(t, session.Driver(), byteRangeCustomID)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s byte-range layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "find-byte-range-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	clickID(t, session.Driver(), byteRangeCloseID)
	clickID(t, session.Driver(), menuScope.At(2))
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact Search top menu layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "find-byte-range-search-menu-top-compact.png")); err != nil {
		t.Fatal(err)
	}
	state.searchMenuOffset = len(menuCommands[2]) - state.searchMenuVisible
	state.invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact Search bottom menu layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "find-byte-range-search-menu-bottom-compact.png")); err != nil {
		t.Fatal(err)
	}
}
