package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func TestCloseCommandDetachesOnlyActiveClone(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pane     int
		shortcut bool
		dirty    bool
	}{
		{name: "menu closes clean clone from the owner", pane: 0},
		{name: "menu closes dirty clone", pane: 1, dirty: true},
		{name: "shortcut closes clean clone", pane: 1, shortcut: true},
		{name: "shortcut closes dirty clone from the owner", pane: 0, shortcut: true, dirty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "shared.txt")
			const disk = "saved\r\n"
			if err := os.WriteFile(path, []byte(disk), 0600); err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			if err := state.space.Open(path); err != nil {
				t.Fatal(err)
			}
			shared := state.space.Current()
			if tc.dirty {
				shared.Editor.Insert("draft ")
			}
			value := shared.Editor.String()
			peer := state.space.NewDocument()
			state.paneOwner[peer] = tc.pane
			state.paneDocs = [2]*document.Document{shared, shared}
			state.splitEnabled = true
			state.activatePane(tc.pane)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			closeTab := func() {
				t.Helper()
				if tc.shortcut {
					if handled, err := driver.ShortcutChord(primary(), 'w'); err != nil || !handled {
						t.Fatalf("Close shortcut: handled=%v, err=%v", handled, err)
					}
				} else {
					clickID(t, driver, menuScope.At(0))
					clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[0], "Close")))
				}
			}
			closeTab()
			if state.pendingClose >= 0 || len(state.closedHistory) != 0 || state.documentIndex(shared) < 0 {
				t.Fatalf("closed or prompted for a shared buffer: pending=%d history=%v", state.pendingClose, state.closedHistory)
			}
			if !state.splitEnabled || state.paneDocs[tc.pane] != peer || state.paneDocs[1-tc.pane] != shared || state.paneOwner[shared] != 1-tc.pane {
				t.Fatal("Close did not retain only the other view's copy")
			}
			if shared.Editor.String() != value || shared.Dirty() != tc.dirty || shared.Editor.CanUndo() != tc.dirty {
				t.Fatal("detaching the clone altered the shared buffer or history")
			}
			if tc.dirty {
				focus := editorID
				if tc.pane == 0 {
					focus = rightEditorID
				}
				clickID(t, driver, focus)
				closeTab()
				if state.pendingClose != state.documentIndex(shared) || state.pendingClose < 0 {
					t.Fatal("closing the last copy of a dirty document did not prompt")
				}
				if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.pendingClose >= 0 || shared.Editor.String() != value {
					t.Fatalf("canceling the last close did not keep the draft: %v", err)
				}
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != disk {
				t.Fatalf("Close changed the saved file: %q, %v", data, err)
			}
		})
	}
}
