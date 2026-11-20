package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestDistractionFreeModeKeepsActiveDraftAndRestoresWorkspace(t *testing.T) {
	for _, exit := range []string{"button", "Escape"} {
		t.Run(exit, func(t *testing.T) {
			state, driver := startDriver(t)
			state.requestFullscreen = driver.SetFullscreen
			left := state.space.Current()
			left.Editor.Insert("left draft")
			right := state.space.NewDocument()
			right.Editor.Insert("right draft")
			selection := text.Range{Start: 2, End: 4}
			right.Editor.SetSelection(selection)
			state.toggleSplit()
			state.folderVisible, state.documentMapVisible = true, true
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			before := driver.Frame()
			monitor := gfx.Rect{Width: 960, Height: 600}
			if err := driver.SetFullscreenMonitor(monitor); err != nil {
				t.Fatal(err)
			}
			index := slices.Index(menuCommands[3], "Distraction Free Mode")
			if index < 0 {
				t.Fatal("View menu does not offer Distraction Free Mode")
			}
			clickID(t, driver, menuScope.At(3))
			clickID(t, driver, menuItemScope.At(index))
			if !driver.Fullscreen() || !state.menuSelected("Distraction Free Mode") {
				t.Fatal("mode did not enter fullscreen")
			}
			assertDistractionFreeLayout(t, driver.Snapshot().Hits, monitor.Width, monitor.Height)
			requireEditorCommandFocus(t, driver, rightEditorID)
			if exit == "Escape" {
				if err := driver.Key(gfx.KeyEscape, 0); err != nil {
					t.Fatal(err)
				}
			} else {
				clickID(t, driver, fullscreenRestoreID)
			}
			requireEditorCommandFocus(t, driver, rightEditorID)
			if driver.Fullscreen() || driver.Frame() != before || state.menuSelected("Distraction Free Mode") || state.activePane != 1 ||
				!state.splitEnabled || state.paneDocument(0) != left || state.paneDocument(1) != right ||
				!state.folderVisible || !state.documentMapVisible || right.Editor.Selection() != selection ||
				left.Editor.String() != "left draft" || right.Editor.String() != "right draft" {
				t.Fatal("leaving Distraction Free Mode changed the previous workspace or drafts")
			}
		})
	}
}

func assertDistractionFreeLayout(t *testing.T, hits []gfx.Hit, width, height float32) {
	t.Helper()
	var editor, restore bool
	for _, hit := range hits {
		switch hit.ID {
		case leftPaneID, rightPaneID, editorID, documentMapID, saveID, splitterID, menuScope.At(3):
			t.Fatalf("unexpected tool or inactive editor in Distraction Free Mode: %+v", hit)
		case rightEditorID:
			editor = true
			if hit.Bounds != (gfx.Rect{X: width / 4, Width: width / 2, Height: height}) {
				t.Fatalf("focused editor bounds = %+v", hit.Bounds)
			}
		case fullscreenRestoreID:
			restore = true
			if hit.Bounds.X < 3*width/4 || hit.Bounds.X+hit.Bounds.Width > width {
				t.Fatalf("restore control overlaps the editor or viewport: %+v", hit.Bounds)
			}
		}
	}
	if !editor || !restore {
		t.Fatalf("editor visible=%v exit control visible=%v", editor, restore)
	}
}

func TestDistractionFreeEscapePreservesHiddenFinder(t *testing.T) {
	state, driver := startDriver(t)
	state.requestFullscreen = driver.SetFullscreen
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	state.query.Insert("cat")
	state.findAllOpenDocuments()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, resultScope.At(0))
	if !state.resultsOpen || state.resultSelected != 0 || len(state.fileResults.Matches) != 2 {
		t.Fatal("fixture did not select a Finder result")
	}
	state.command(distractionFreeCommand)
	if !driver.Fullscreen() || !state.distractionFree {
		t.Fatal("did not enter Distraction Free Mode")
	}
	state.command("Find...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.searchOpen || !driver.Fullscreen() || !state.resultsOpen || state.resultSelected != 0 {
		t.Fatalf("first Escape should close Find and keep fullscreen and Finder: search=%t fullscreen=%t results=%t selected=%d",
			state.searchOpen, driver.Fullscreen(), state.resultsOpen, state.resultSelected)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Fullscreen() || !state.resultsOpen || state.resultSelected != 0 || len(state.fileResults.Matches) != 2 {
		t.Fatal("second Escape did not exit fullscreen and restore the existing Finder")
	}
	clickID(t, driver, resultScope.At(1))
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || doc.Editor.String() != "cat cat" {
		t.Fatal("restored Finder did not navigate to the second original result")
	}
}

func TestDistractionFreeModeFollowsObservedFullscreenState(t *testing.T) {
	state, driver := startDriver(t)
	var requested []bool
	state.requestFullscreen = func(value bool) error { requested = append(requested, value); return nil }
	state.command("Distraction Free Mode")
	if !slices.Equal(requested, []bool{true}) || state.menuSelected("Distraction Free Mode") {
		t.Fatalf("request=%v active=%v before host confirmation", requested, state.menuSelected("Distraction Free Mode"))
	}
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	if !state.menuSelected("Distraction Free Mode") {
		t.Fatal("host confirmation did not activate the mode")
	}
	if err := driver.SetFullscreen(false); err != nil {
		t.Fatal(err)
	}
	if state.menuSelected("Distraction Free Mode") {
		t.Fatal("system exit left distraction-free layout active")
	}
	state.requestFullscreen = func(bool) error { return errors.New("host rejected request") }
	state.command("Distraction Free Mode")
	if state.menuSelected("Distraction Free Mode") || !strings.Contains(state.notice, "host rejected request") {
		t.Fatalf("rejected request: active=%v notice=%q", state.menuSelected("Distraction Free Mode"), state.notice)
	}
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	if state.menuSelected("Distraction Free Mode") || !state.menuSelected("Toggle Full Screen Mode") {
		t.Fatal("normal system fullscreen inherited an old distraction-free request")
	}
}
