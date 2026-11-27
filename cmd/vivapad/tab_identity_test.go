package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestTabStripRejectsReleaseOnReorderedDocument(t *testing.T) {
	for _, layout := range []string{"single", "split"} {
		for _, action := range []string{"Select", "Pin", "Close"} {
			t.Run(layout+"/"+action, func(t *testing.T) {
				state, driver := startDriver(t)
				first, second := state.space.Current(), state.space.NewDocument()
				first.Name, second.Name = "zeta.txt", "alpha.txt"
				state.space.Select(state.documentIndex(first))
				state.showInActivePane(first)
				if layout == "split" {
					peer := state.space.NewDocument()
					state.paneOwner[peer] = 1
					state.space.Select(state.documentIndex(first))
					state.toggleSplit()
					state.activatePane(0)
				}
				if err := driver.Resize(1000, 600); err != nil {
					t.Fatal(err)
				}
				labels := func(name string) string {
					if action == "Select" {
						return name
					}
					return action + " " + name
				}
				bounds := func(label string) gfx.Rect {
					tab, pin, close := tabScope, tabPinScope, tabCloseScope
					if layout == "split" {
						tab, pin, close = leftTabScope, leftPinScope, leftCloseScope
					}
					for _, hit := range driver.Snapshot().Hits {
						scope := hit.ID.Scope()
						if hit.Label == label && (scope == tab || scope == pin || scope == close) {
							return hit.Bounds
						}
					}
					t.Fatalf("missing tab control %q", label)
					return gfx.Rect{}
				}
				pressed := bounds(labels(first.Name))
				x, y := pressed.X+pressed.Width/2, pressed.Y+pressed.Height/2
				if err := driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
					t.Fatal(err)
				}
				if err := state.sortWindowTabs(windowSortCommand{column: 0}); err != nil {
					t.Fatal(err)
				}
				state.invalidator.Invalidate()
				if err := driver.Pump(); err != nil {
					t.Fatal(err)
				}
				if bounds(labels(second.Name)) != pressed {
					t.Fatal("sort did not replace the pressed tab or button")
				}
				if err := driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
					t.Fatal(err)
				}
				if state.space.Current() != first || state.documentIndex(second) < 0 || state.isPinned(second) || state.pendingClose >= 0 {
					t.Fatalf("stale %s release acted on a replacement tab", action)
				}
				if err := driver.Click(x, y); err != nil {
					t.Fatal(err)
				}
				switch action {
				case "Select":
					if state.space.Current() != second || driver.Snapshot().Focused != editorID {
						t.Fatal("fresh tab click did not select and focus the displayed document")
					}
				case "Pin":
					if !state.isPinned(second) || state.isPinned(first) {
						t.Fatal("fresh pin click did not pin only the displayed document")
					}
				case "Close":
					if state.documentIndex(second) >= 0 || state.documentIndex(first) < 0 {
						t.Fatal("fresh close click did not close only the displayed document")
					}
				}
			})
		}
	}
}
