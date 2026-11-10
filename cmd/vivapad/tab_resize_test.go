package main

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestActiveTabRemainsVisibleAfterNarrowResize(t *testing.T) {
	state := newModel()
	for range 9 {
		state.showInActivePane(state.space.NewDocument())
	}
	active := state.space.Active
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	visible := false
	for _, hit := range driver.Snapshot().Hits {
		visible = visible || hit.ID == tabScope.At(active) && hit.Role == ui.RoleTab && hit.Selected
	}
	if !visible {
		t.Fatalf("active tab %d is hidden after resize; offset=%d", active, state.tabOffset)
	}
	previous := state.tabOffset
	clickID(t, driver, tabLeftID)
	if state.tabOffset != previous-1 {
		t.Fatalf("tab navigation did not move: before=%d after=%d", previous, state.tabOffset)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.tabOffset != previous-1 {
		t.Fatalf("normal repaint undid manual tab navigation: %d", state.tabOffset)
	}
}

func TestActiveSplitViewTabRemainsVisibleAfterNarrowResize(t *testing.T) {
	state := newModel()
	state.toggleSplit()
	state.activePane = 1
	for range 9 {
		state.showInActivePane(state.space.NewDocument())
	}
	active := state.space.Active
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	visible := false
	for _, hit := range driver.Snapshot().Hits {
		visible = visible || hit.ID == rightTabScope.At(active) && hit.Role == ui.RoleTab && hit.Selected
	}
	if !visible {
		t.Fatalf("active right-view tab %d is hidden after resize; offset=%d", active, state.paneTabOffset[1])
	}
}
