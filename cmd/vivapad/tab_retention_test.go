package main

import (
	"slices"
	"testing"
)

func TestLeavingSplitDropsTabLayoutReferences(t *testing.T) {
	state, driver := startDriver(t)
	state.space.NewDocument()
	state.toggleSplit()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for pane, anchor := range state.paneTabAnchor {
		if anchor == nil || len(state.paneTabScratch[pane]) == 0 {
			t.Fatalf("split pane %d did not populate tab layout", pane)
		}
	}
	current := state.space.Current()
	state.toggleSplit()
	if state.space.Current() != current || len(state.space.Documents) != 2 {
		t.Fatal("hiding the split changed the live tabs")
	}
	for pane, anchor := range state.paneTabAnchor {
		if anchor != nil || state.paneTabScratch[pane] != nil {
			t.Fatalf("hidden split pane %d retained documents in its tab layout", pane)
		}
	}
}

func TestCloseDropsTabLayoutReferencesBeforeNextPaint(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	right := state.space.NewDocument()
	state.toggleSplit()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(state.paneTabScratch[1], right) || state.paneTabAnchor[1] != right {
		t.Fatal("closing tab was not cached in the right pane")
	}
	state.closeTab(state.documentIndex(right))
	if state.space.Current() != left {
		t.Fatal("closing the right tab did not select the survivor")
	}
	for pane, anchor := range state.paneTabAnchor {
		if anchor == right || slices.Contains(state.paneTabScratch[pane], right) {
			t.Fatalf("pane %d still holds the closed document", pane)
		}
	}
}
