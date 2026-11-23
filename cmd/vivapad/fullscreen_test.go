package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestFullScreenViewMenuRestoresFrameAndEditingState(t *testing.T) {
	state, driver := startDriver(t)
	state.requestFullscreen = driver.SetFullscreen
	doc := state.space.Current()
	doc.Editor.Insert("draft text")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 1, End: 5})
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	before := driver.Frame()
	monitor := gfx.Rect{X: -1200, Width: 1200, Height: 720}
	if err := driver.SetFullscreenMonitor(monitor); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	row := syncScrollHit(t, driver, menuItemScope.At(viewCommandIndex("Toggle Full Screen Mode")))
	if row.Y+row.Height > 400-statusHeight || row.Height < 11 {
		t.Fatalf("fullscreen row clipped: %+v", row)
	}
	clickID(t, driver, menuItemScope.At(viewCommandIndex("Toggle Full Screen Mode")))
	if !state.fullscreen || !driver.Fullscreen() || driver.Frame() != monitor || !state.menuSelected("Toggle Full Screen Mode") {
		t.Fatalf("fullscreen entry = observed:%v frame:%+v", state.fullscreen, driver.Frame())
	}
	menuVisible, toolbarVisible, restore := false, false, false
	for _, hit := range driver.Snapshot().Hits {
		menuVisible = menuVisible || hit.ID == menuScope.At(3)
		toolbarVisible = toolbarVisible || hit.ID == saveID
		if hit.ID == fullscreenRestoreID {
			restore = true
			if hit.Bounds.X < monitor.Width-35 || hit.Bounds.Y != 0 || hit.Bounds.X+hit.Bounds.Width > monitor.Width {
				t.Fatalf("restore control overlaps tabs or leaves viewport: %+v", hit.Bounds)
			}
		}
	}
	if menuVisible || toolbarVisible || !restore {
		t.Fatalf("fullscreen chrome: menu=%v toolbar=%v restore=%v", menuVisible, toolbarVisible, restore)
	}
	clickID(t, driver, fullscreenRestoreID)
	if state.fullscreen || driver.Fullscreen() || driver.Frame() != before || doc.Editor.String() != "draft text" || doc.Editor.Selection() != (text.Range{Start: 1, End: 5}) || doc.Editor.CanUndo() {
		t.Fatalf("fullscreen exit lost placement or editing state: frame=%+v", driver.Frame())
	}
}

func TestFullScreenSystemTransitionAndRejectedRequest(t *testing.T) {
	state, driver := startDriver(t)
	state.requestFullscreen = driver.SetFullscreen
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	if !state.fullscreen {
		t.Fatal("system fullscreen transition did not update the model")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.fullscreen || driver.Fullscreen() {
		t.Fatal("Escape did not leave fullscreen")
	}
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	state.command("Toggle Full Screen Mode")
	if state.fullscreen || driver.Fullscreen() {
		t.Fatal("command did not exit system-initiated fullscreen")
	}
	rejected := errors.New("fullscreen unavailable")
	state.requestFullscreen = func(bool) error { return rejected }
	state.command("Toggle Full Screen Mode")
	if state.fullscreen || !strings.Contains(state.notice, rejected.Error()) {
		t.Fatalf("rejected fullscreen request changed state: %v, %q", state.fullscreen, state.notice)
	}
}

func TestFullScreenCommandsReturnFocusToTheEditor(t *testing.T) {
	for _, exit := range []string{"Escape", "restore button"} {
		t.Run(exit, func(t *testing.T) {
			state, driver := startDriver(t)
			state.requestFullscreen = driver.SetFullscreen
			clickID(t, driver, editorID)
			if err := driver.Type("draft"); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			doc.Editor.MoveTo(2, false)
			clickID(t, driver, menuScope.At(3))
			clickID(t, driver, menuItemScope.At(viewCommandIndex("Toggle Full Screen Mode")))
			requireEditorCommandFocus(t, driver, editorID)
			if exit == "Escape" {
				if err := driver.Key(gfx.KeyEscape, 0); err != nil {
					t.Fatal(err)
				}
			} else {
				clickID(t, driver, fullscreenRestoreID)
			}
			requireEditorCommandFocus(t, driver, editorID)
			if state.fullscreen {
				t.Fatal("fullscreen remained active")
			}
			if err := driver.Type("!"); err != nil || doc.Editor.String() != "dr!aft" {
				t.Fatalf("first key after fullscreen = %q, %v", doc.Editor.String(), err)
			}
		})
	}
}

func TestFullScreenCompactSplitDocksKeepRestoreReachable(t *testing.T) {
	state, driver := startDriver(t)
	state.requestFullscreen = driver.SetFullscreen
	left := state.space.Current()
	for range 10 {
		state.space.NewDocument()
	}
	right := state.space.Current()
	state.splitEnabled, state.activePane = true, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.folderVisible, state.documentMapVisible = true, true
	if err := driver.SetFullscreenMonitor(gfx.Rect{Width: 640, Height: 400}); err != nil {
		t.Fatal(err)
	}
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	restore := syncScrollHit(t, driver, fullscreenRestoreID)
	if restore.X+restore.Width > 640 || restore.Y+restore.Height > tabHeight {
		t.Fatalf("restore control outside top strip: %+v", restore)
	}
	for _, hit := range driver.Snapshot().Hits {
		switch hit.ID {
		case rightPaneID, rightEditorID, documentMapID, rightTabsBackID, rightTabsNextID:
			if hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight && hit.ID != rightPaneID {
				t.Fatalf("fullscreen dock/tab clipped: %+v", hit)
			}
		}
		if hit.ID.Scope() == rightTabScope && hit.Bounds.X+hit.Bounds.Width > restore.X {
			t.Fatalf("right tab overlaps restore button: %+v", hit.Bounds)
		}
	}
	clickID(t, driver, fullscreenRestoreID)
	if state.fullscreen || driver.Fullscreen() {
		t.Fatal("restore did not leave compact fullscreen")
	}
}

func TestCaptureFullScreenWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_FULLSCREEN_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_FULLSCREEN_SHOTS to capture fullscreen UI")
	}
	state := newModel()
	state.space.Current().Editor.Insert("fullscreen draft")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	state.requestFullscreen = session.Driver().SetFullscreen
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), menuScope.At(3))
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact menu layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "fullscreen-compact-menu.png")); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), menuItemScope.At(viewCommandIndex("Toggle Full Screen Mode")))
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("fullscreen layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "fullscreen-active.png")); err != nil {
		t.Fatal(err)
	}
}
