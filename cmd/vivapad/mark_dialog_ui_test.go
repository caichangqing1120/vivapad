package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

func TestMarkMenuOpensCompactPanelAndRunsActionsWithoutEditing(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("cat cat\ndog")
	doc.Editor.ClearHistory()
	setMarkQuery(state, "cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(10))
	if !state.markDialogOpen || len(state.markedSpans) != 0 {
		t.Fatalf("Mark menu did not open the panel without executing: open=%v marks=%+v", state.markDialogOpen, state.markedSpans)
	}
	for _, id := range []gfx.ID{markPanelID, queryID, markBookmarkID, markPurgeID, markInSelectionID, markWrapID,
		markModeScope.At(0), markModeScope.At(1), markModeScope.At(2), markAllID, markClearID, markCopyID, markCloseID} {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == id {
				found = true
				if hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y < 0 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("Mark control %+v clipped: %+v", id, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("Mark control %+v missing", id)
		}
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact Mark layout: %+v", problems)
	}
	clickID(t, driver, queryID)
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if len(state.markedSpans) != 2 || doc.Editor.String() != "cat cat\ndog" || doc.Editor.CanUndo() {
		t.Fatalf("Enter did not mark without editing: %+v", state.markedSpans)
	}
	clickID(t, driver, markAllID)
	if len(state.markedSpans) != 2 {
		t.Fatalf("Mark All button changed spans: %+v", state.markedSpans)
	}
	clickID(t, driver, markModeScope.At(2))
	if state.markMode != "regex" || !state.searchOptions.Regex {
		t.Fatal("regex mode did not update shared search options")
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact regex Mark layout: %+v", problems)
	}
	clickID(t, driver, markModeScope.At(0))
	clickID(t, driver, markBookmarkID)
	clickID(t, driver, markPurgeID)
	if !state.markBookmarkLine || !state.markPurge {
		t.Fatal("Mark options did not toggle")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.markDialogOpen || len(state.markedSpans) != 2 {
		t.Fatalf("Escape closed marks or left panel: %v, open=%v marks=%+v", err, state.markDialogOpen, state.markedSpans)
	}
}

func TestMarkPanelKeepsEditorInteractiveOutsideItsBounds(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("cat")
	state.command("Mark...")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(0, false)
	if err := driver.Click(90, 350); err != nil || doc.Editor.Caret() != 0 {
		t.Fatalf("clicking panel background moved caret: %v, %d", err, doc.Editor.Caret())
	}
	if err := driver.Click(620, 236); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("!"); err != nil || !state.markDialogOpen || doc.Editor.String() != "cat!" {
		t.Fatalf("editor blocked by modeless Mark panel: %v, open=%v, text=%q", err, state.markDialogOpen, doc.Editor.String())
	}
}

func TestOpeningFindClosesMarkSharedQuerySurface(t *testing.T) {
	state, driver := startDriver(t)
	state.command("Mark...")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchID)
	if state.markDialogOpen || !state.searchOpen {
		t.Fatalf("Find and Mark both own query: mark=%v find=%v", state.markDialogOpen, state.searchOpen)
	}
	count := 0
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == queryID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("shared Find What has %d active controls", count)
	}
}

func TestCaptureMarkPanelWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_MARK_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_MARK_SHOTS to capture the Mark panel")
	}
	state := newModel()
	state.space.Current().Editor.Insert("cat cat\ndog cat")
	setMarkQuery(state, "cat")
	state.command("Mark...")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 800}, {"compact", 640, 400},
	} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s Mark layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "mark-dialog-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	state.markMode = "regex"
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact regex Mark layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "mark-dialog-regex-compact.png")); err != nil {
		t.Fatal(err)
	}
}
