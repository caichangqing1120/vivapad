package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"vivapad/internal/document"
)

func TestSplitZoomIsIndependentUntilSyncAndDoesNotEditText(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	left.Editor.ClearHistory()
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	right.Editor.ClearHistory()
	state.splitEnabled = true
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.activePane = 0
	state.space.Select(state.documentIndex(left))
	state.command("Zoom In")
	if state.fontSize != 14 || state.subFontSize != 13 {
		t.Fatalf("main zoom leaked to sub view: %d %d", state.fontSize, state.subFontSize)
	}
	state.activatePane(1)
	state.command("Zoom Out")
	if state.fontSize != 14 || state.subFontSize != 12 {
		t.Fatalf("sub zoom leaked to main view: %d %d", state.fontSize, state.subFontSize)
	}
	state.command("Synchronize Zoom Across Views")
	if !state.zoomSync || state.fontSize != 12 || state.subFontSize != 12 {
		t.Fatalf("sync did not align to active view: %d %d, %v", state.fontSize, state.subFontSize, state.zoomSync)
	}
	state.command("Zoom In")
	if state.fontSize != 13 || state.subFontSize != 13 {
		t.Fatal("synchronized zoom did not move both views")
	}
	state.command("Synchronize Zoom Across Views")
	state.activatePane(0)
	state.command("Zoom In")
	if state.fontSize != 14 || state.subFontSize != 13 {
		t.Fatal("turning sync off did not restore independent zoom")
	}
	if left.Editor.String() != "left" || right.Editor.String() != "right" || left.Editor.CanUndo() || right.Editor.CanUndo() {
		t.Fatal("zoom changed document text or history")
	}
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	row := revealViewMenuItem(t, driver, viewCommandIndex("Synchronize Zoom Across Views"))
	if row.Y+row.Height > 400-statusHeight || row.Height < 20 {
		t.Fatalf("zoom-sync View row clipped: %+v", row)
	}
	clickID(t, driver, menuItemScope.At(viewCommandIndex("Synchronize Zoom Across Views")))
	if !state.zoomSync || state.subFontSize != 14 {
		t.Fatal("View menu did not align zoom")
	}
	clickID(t, driver, menuScope.At(3))
	revealViewMenuItem(t, driver, viewCommandIndex("Synchronize Zoom Across Views"))
	checked := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(viewCommandIndex("Synchronize Zoom Across Views")) {
			checked = hit.Checked
		}
	}
	if !checked {
		t.Fatal("synchronized zoom is not checked in View menu")
	}
}

func TestZoomShortcutUsesActiveSplitViewAndSettingsPersist(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings", "session.json")
	state := newStartupModel(nil, path)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	state.splitEnabled, state.activePane = true, 1
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.space.Select(state.documentIndex(right))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, rightEditorID)
	if handled, err := driver.ShortcutChord(primary(), '+'); err != nil || !handled {
		t.Fatalf("zoom shortcut = handled:%v err:%v", handled, err)
	}
	if state.fontSize != 13 || state.subFontSize != 14 {
		t.Fatalf("shortcut changed wrong pane: %d %d", state.fontSize, state.subFontSize)
	}
	state.command("Synchronize Zoom Across Views")
	restored := newStartupModel(nil, path)
	if restored.fontSize != 14 || restored.subFontSize != 14 || !restored.zoomSync {
		t.Fatalf("restored view zoom = %d %d sync=%v", restored.fontSize, restored.subFontSize, restored.zoomSync)
	}
}

func TestCaptureSplitZoomWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_SPLIT_ZOOM_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_SPLIT_ZOOM_SHOTS to capture split zoom")
	}
	state := newModel()
	left := state.space.Current()
	left.Name = "main.go"
	left.Editor.Insert("package main\nfunc main() {}\n")
	right := state.space.NewDocument()
	right.Name = "sub.go"
	right.Editor.Insert("package sub\nfunc second() {}\n")
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 0
	state.space.Select(state.documentIndex(left))
	state.fontSize, state.subFontSize = 18, 11
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
			clickID(t, session.Driver(), menuScope.At(3))
			revealViewMenuItem(t, session.Driver(), 27)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "split-zoom-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}
