package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestDoubleClickEmptyTabBarCreatesNewDocument(t *testing.T) {
	state, driver := startDriver(t)
	if len(state.space.Documents) != 1 || state.space.Current().Name != "new 1" {
		t.Fatalf("initial documents = %d (%q)", len(state.space.Documents), state.space.Current().Name)
	}

	var bar gfx.Rect
	var found bool
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == tabBarID {
			bar = hit.Bounds
			found = true
			break
		}
	}
	if !found {
		t.Fatal("tabBarID hit region not found")
	}

	// Double-clicking empty space to the right of existing tabs creates a new tab and focuses editor.
	targetX := bar.X + bar.Width - 40
	targetY := bar.Y + bar.Height/2
	if err := driver.DoubleClick(targetX, targetY); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 2 || state.space.Current().Name != "new 2" {
		t.Fatalf("after empty tab bar double-click: count=%d active=%q", len(state.space.Documents), state.space.Current().Name)
	}
	requireEditorCommandFocus(t, driver, editorID)

	// Double-clicking directly on an existing tab must NOT create another tab.
	firstTabID := tabScope.At(state.tabHitIndex(state.space.Documents[0]))
	var firstTab gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == firstTabID {
			firstTab = hit.Bounds
			break
		}
	}
	if err := driver.DoubleClick(firstTab.X+24, firstTab.Y+firstTab.Height/2); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 2 || state.space.Current().Name != "new 1" {
		t.Fatalf("after double-clicking existing tab: count=%d active=%q", len(state.space.Documents), state.space.Current().Name)
	}

	// Clicking the quick '+' button next to the rightmost tab also creates a new tab.
	clickID(t, driver, tabNewID)
	if len(state.space.Documents) != 3 || state.space.Current().Name != "new 3" {
		t.Fatalf("after clicking tabNewID: count=%d active=%q", len(state.space.Documents), state.space.Current().Name)
	}
}

func TestDoubleClickEmptySplitPaneTabBarCreatesNewDocumentInPane(t *testing.T) {
	state, driver := startDriver(t)
	state.showInActivePane(state.space.NewDocument())
	state.toggleSplit()
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}

	var rightBar gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == rightPaneID {
			rightBar = hit.Bounds
			break
		}
	}
	if rightBar.Width <= 0 {
		t.Fatal("rightPaneID hit region not found")
	}

	if err := driver.DoubleClick(rightBar.X+rightBar.Width-24, rightBar.Y+rightBar.Height/2); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 3 || state.activePane != 1 || state.paneDocument(1).Name != "new 3" {
		t.Fatalf("after split tab bar double-click: count=%d pane=%d doc=%q", len(state.space.Documents), state.activePane, state.paneDocument(1).Name)
	}
	requireEditorCommandFocus(t, driver, rightEditorID)
}
