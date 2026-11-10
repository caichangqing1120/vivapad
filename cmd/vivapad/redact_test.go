package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func TestWholeBufferRedactionPreservesLinesAndClearsSourceMarkers(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a🙂\n猫b")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 5})
	doc.Editor.ClearHistory()
	state.command("Redact Selection")
	if doc.Editor.String() != "██\n██" || len(state.bookmarkLines(doc)) != 0 {
		t.Fatalf("redacted document = %q, marks=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	state.command("Undo")
	if doc.Editor.String() != "a🙂\n猫b" || doc.Editor.CanUndo() || len(state.bookmarkLines(doc)) != 0 {
		t.Fatalf("redaction undo = %q, marks=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestRedactSelectionShiftMenuUsesLegacyASCIIAndCompactReachability(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("Aب")
	doc.SetEncoding(document.DOS720)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	clickID(t, driver, menuScope.At(1))
	item := revealEditMenuItem(t, driver, 24)
	if item.Y < menuHeight || item.Y+item.Height > 400-statusHeight {
		t.Fatalf("On Selection row clipped: %+v", item)
	}
	clickID(t, driver, menuItemScope.At(24))
	if err := driver.KeyDown(ui.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, onSelectionScope.At(2))
	if err := driver.KeyUp(ui.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != ".." || doc.Encoding() != document.DOS720 {
		t.Fatalf("Shift redaction = %q, encoding=%d", doc.Editor.String(), doc.Encoding())
	}
}

func TestRedactSelectionRespectsReadOnlySplitAndEmptySelection(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.splitEnabled, state.activePane = true, 1
	state.space.Select(state.documentIndex(right))
	right.Editor.SetSelection(text.Range{Start: 0, End: 5})
	state.readOnly[right] = true
	state.command("Redact Selection")
	if right.Editor.String() != "right" || left.Editor.String() != "left" || state.notice != "Read-only document" {
		t.Fatalf("read-only redaction = %q, notice=%q", right.Editor.String(), state.notice)
	}
	delete(state.readOnly, right)
	right.Editor.SetSelection(text.Range{Start: 0, End: 0})
	right.Editor.ClearHistory()
	state.command("Redact Selection")
	if right.Editor.String() != "right" || right.Editor.CanUndo() {
		t.Fatal("empty redaction mutated the document")
	}
	right.Editor.SetSelection(text.Range{Start: 0, End: 5})
	state.command("Redact Selection")
	if right.Editor.String() != "█████" || left.Editor.String() != "left" {
		t.Fatalf("active-pane redaction = %q, left=%q", right.Editor.String(), left.Editor.String())
	}
}

func TestRedactRectangularSelectionIsAtomicAndUndoRestoresRectangle(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.MoveTo(0, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.ClearHistory()
	state.command("Redact Selection")
	if doc.Editor.String() != "█b\n█d" {
		t.Fatalf("rectangle redaction = %q", doc.Editor.String())
	}
	state.command("Undo")
	if doc.Editor.String() != "ab\ncd" || !state.rectangles[0].active || doc.Editor.CanUndo() {
		t.Fatalf("rectangle undo = %q, selection=%+v", doc.Editor.String(), state.rectangles[0])
	}
}

func TestCaptureRedactSelectionWhenRequested(t *testing.T) {
	directory := os.Getenv("VIVAPAD_REDACT_SHOTS")
	if directory == "" {
		t.Skip("set VIVAPAD_REDACT_SHOTS to capture redaction")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a🙂\n猫b")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 5})
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), menuScope.At(1))
	revealEditMenuItem(t, session.Driver(), 24)
	clickID(t, session.Driver(), menuItemScope.At(24))
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("redaction menu layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(directory, "redact-selection-menu-compact.png")); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), onSelectionScope.At(2))
	if doc.Editor.String() != "██\n██" {
		t.Fatalf("redaction result = %q", doc.Editor.String())
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("redaction result layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(directory, "redact-selection-result-compact.png")); err != nil {
		t.Fatal(err)
	}
}

func TestRedactSubmenuClosesWhenSwitchingTopLevelMenu(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	revealEditMenuItem(t, driver, 24)
	clickID(t, driver, menuItemScope.At(24))
	if !state.onSelectionMenuOpen {
		t.Fatal("On Selection submenu did not open")
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuScope.At(1))
	if state.onSelectionMenuOpen {
		t.Fatal("stale On Selection submenu reopened")
	}
}
