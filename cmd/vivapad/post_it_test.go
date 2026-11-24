package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestPostItKeepsBothEditorsAndRestoresTheWindow(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left draft")
	right := state.space.NewDocument()
	right.Editor.Insert("right draft")
	right.Editor.SetSelection(text.Range{Start: 6, End: 11})
	state.toggleSplit()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	var levels, decorations []bool
	var complete func(error)
	state.requestAlwaysOnTop = func(enabled bool) error { levels = append(levels, enabled); return nil }
	state.requestDecoration = func(decorated bool, done func(error)) error {
		decorations, complete = append(decorations, decorated), done
		return nil
	}
	index := slices.Index(menuCommands[3], postItCommand)
	if index < 0 {
		t.Fatal("View menu does not offer Post-It")
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(index))
	if state.menuSelected(postItCommand) || complete == nil || !slices.Equal(decorations, []bool{false}) {
		t.Fatal("Post-It became active before the native transition finished")
	}
	complete(nil)
	requireEditorCommandFocus(t, driver, rightEditorID)
	hits := driver.Snapshot().Hits
	visible := map[gfx.ID]bool{}
	for _, hit := range hits {
		visible[hit.ID] = true
		if hit.ID == postItRestoreID && (hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400) {
			t.Fatalf("restore button is outside the viewport: %v", hit.Bounds)
		}
	}
	if !state.menuSelected(postItCommand) || !visible[postItRestoreID] || !visible[editorID] || !visible[rightEditorID] ||
		visible[menuScope.At(3)] || visible[saveID] {
		t.Fatal("Post-It did not keep both editors and hide the normal chrome")
	}
	clickID(t, driver, postItRestoreID)
	if !state.menuSelected(postItCommand) || !slices.Equal(decorations, []bool{false, true}) {
		t.Fatal("Post-It hid its restore button before native confirmation")
	}
	complete(nil)
	requireEditorCommandFocus(t, driver, rightEditorID)
	if state.menuSelected(postItCommand) || state.fullscreen || !slices.Equal(levels, []bool{true, false}) ||
		!state.splitEnabled || state.paneDocument(0) != left || state.paneDocument(1) != right || state.activePane != 1 {
		t.Fatalf("workspace did not restore; level requests=%v", levels)
	}
	if err := driver.Type("edit"); err != nil || right.Editor.String() != "right edit" || left.Editor.String() != "left draft" {
		t.Fatalf("first edit did not replace the original right selection: %q, %q, %v", left.Editor.String(), right.Editor.String(), err)
	}
	state.command("Undo")
	if right.Editor.String() != "right draft" || left.Editor.String() != "left draft" {
		t.Fatal("Undo did not restore the original split documents")
	}
}

func TestPostItF12InFullscreenKeepsExistingTopmost(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	state.alwaysOnTop = true
	state.requestAlwaysOnTop = func(bool) error { t.Error("changed an existing topmost level"); return nil }
	state.requestDecoration = func(bool, func(error)) error { t.Error("changed native decoration in fullscreen"); return nil }
	if err := driver.Key(gfx.KeyF12, 0); err != nil || !state.menuSelected(postItCommand) {
		t.Fatalf("F12 did not enter Post-It in fullscreen: %v", err)
	}
	clickID(t, driver, postItRestoreID)
	if !state.fullscreen || !driver.Fullscreen() || state.menuSelected(postItCommand) || !state.alwaysOnTop {
		t.Fatal("Post-It exit changed the pre-existing fullscreen or topmost mode")
	}
}

func TestPostItRollsBackFailedNativeRequests(t *testing.T) {
	state, _ := startDriver(t)
	var levels []bool
	rejected := errors.New("frame request refused")
	state.requestAlwaysOnTop = func(enabled bool) error { levels = append(levels, enabled); return nil }
	state.requestDecoration = func(_ bool, done func(error)) error { done(rejected); return nil }
	state.command(postItCommand)
	if state.menuSelected(postItCommand) || !slices.Equal(levels, []bool{true, false}) || !strings.Contains(state.notice, rejected.Error()) {
		t.Fatalf("failed entry left the interface changed: level=%v, notice=%q", levels, state.notice)
	}
	state.requestDecoration = func(decorated bool, done func(error)) error {
		if decorated {
			return rejected
		}
		done(nil)
		return nil
	}
	state.command(postItCommand)
	state.command(postItCommand)
	if !state.menuSelected(postItCommand) || !slices.Equal(levels, []bool{true, false, true, false, true}) {
		t.Fatalf("failed restore did not retain Post-It: level=%v, notice=%q", levels, state.notice)
	}
	state.requestDecoration = func(_ bool, done func(error)) error { done(nil); return nil }
	state.command(postItCommand)
	if state.menuSelected(postItCommand) || levels[len(levels)-1] {
		t.Fatal("retry did not restore the window")
	}
}
