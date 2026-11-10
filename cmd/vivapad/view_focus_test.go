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

func TestFocusOnAnotherViewKeepsBothEditorsIntact(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left text")
	left.Editor.ClearHistory()
	left.Editor.SetSelection(text.Range{Start: 1, End: 4})
	right := state.space.NewDocument()
	right.Editor.Insert("right text")
	right.Editor.ClearHistory()
	right.Editor.SetSelection(text.Range{Start: 2, End: 5})
	state.splitEnabled = true
	state.activePane = 0
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.space.Select(state.documentIndex(left))
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		pane  int
		focus ui.ID
		doc   *document.Document
	}{{1, rightEditorID, right}, {0, editorID, left}} {
		clickID(t, driver, menuScope.At(3))
		hit := syncScrollHit(t, driver, menuItemScope.At(24))
		if hit.Y+hit.Height > 400-statusHeight || hit.Height < 12 {
			t.Fatalf("View focus row clipped: %+v", hit)
		}
		clickID(t, driver, menuItemScope.At(24))
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		if state.activePane != want.pane || state.space.Current() != want.doc || driver.Snapshot().Focused != want.focus {
			t.Fatalf("focus target: pane=%d doc=%p focus=%v", state.activePane, state.space.Current(), driver.Snapshot().Focused)
		}
	}
	if left.Editor.String() != "left text" || right.Editor.String() != "right text" || left.Editor.Selection() != (text.Range{Start: 1, End: 4}) || right.Editor.Selection() != (text.Range{Start: 2, End: 5}) || left.Editor.CanUndo() || right.Editor.CanUndo() {
		t.Fatal("view focus changed editing state")
	}
}

func TestFocusOnAnotherViewWithoutSplitKeepsOneEditor(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(24))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.splitEnabled || state.activePane != 0 || driver.Snapshot().Focused != editorID {
		t.Fatalf("single view focus = split:%v pane:%d focus:%v", state.splitEnabled, state.activePane, driver.Snapshot().Focused)
	}
}

func TestCaptureViewFocusWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_VIEW_FOCUS_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_VIEW_FOCUS_SHOTS to capture View menu")
	}
	state := newModel()
	left := state.space.Current()
	left.Name = "left.go"
	left.Editor.Insert("package main\nfunc main() {}\n")
	right := state.space.NewDocument()
	right.Name = "right.json"
	right.Editor.Insert("{\"active\":true}\n")
	state.splitEnabled = true
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.space.Select(state.documentIndex(left))
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
		clickID(t, session.Driver(), menuScope.At(3))
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "view-focus-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), menuScope.At(3))
	}
}
