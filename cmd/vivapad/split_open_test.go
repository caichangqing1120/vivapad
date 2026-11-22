package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func TestReopeningFilePreservesSplitOwnership(t *testing.T) {
	for _, route := range []string{"picker", "encoding picker", "recent", "drop", "system", "mixed batch", "current clone"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			path, newPath := filepath.Join(root, "loaded.txt"), filepath.Join(root, "new.txt")
			original := []byte{0xd6, 0xd0, '\r', '\n'}
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			if err := state.space.OpenWithEncoding(path, document.GB18030); err != nil {
				t.Fatal(err)
			}
			loaded := state.space.Current()
			loaded.Editor.Insert("draft ")
			right := state.space.NewDocument()
			right.Editor.Insert("independent")
			state.paneOwner[loaded], state.paneOwner[right] = 0, 1
			state.paneDocs = [2]*document.Document{loaded, right}
			state.splitEnabled = true
			if route == "current clone" {
				state.paneDocs[1] = loaded
			}
			state.activatePane(1)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, rightEditorID)
			selection, value := loaded.Editor.Selection(), loaded.Editor.String()

			switch route {
			case "picker", "encoding picker", "mixed batch", "current clone":
				paths := []string{path}
				if route == "mixed batch" {
					if err := os.WriteFile(newPath, []byte("new bytes"), 0600); err != nil {
						t.Fatal(err)
					}
					paths = append(paths, newPath, filepath.Join(root, "missing.txt"))
				}
				if err := driver.QueueOpenFiles(paths, nil); err != nil {
					t.Fatal(err)
				}
				clickID(t, driver, menuScope.At(0))
				if route == "encoding picker" {
					clickMenuLabel(t, driver, "Open as GB18030...")
				} else {
					clickMenuLabel(t, driver, "Open...")
				}
			case "recent":
				state.rememberRecent(loaded)
				clickID(t, driver, menuScope.At(0))
				clickMenuLabel(t, driver, recentFilesCommand)
				clickID(t, driver, recentScope.At(0))
			case "drop":
				accepted, err := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, X: 500, Y: 150, Paths: []string{path}})
				if err != nil || !accepted {
					t.Fatalf("drop accepted=%v, err=%v", accepted, err)
				}
			case "system":
				state.queueSystemDocument(path)
			}
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			wantActive, wantRight, wantTabs := 0, right, 2
			switch route {
			case "mixed batch":
				wantActive, wantRight, wantTabs = 1, state.space.Current(), 3
				if wantRight.Name != "new.txt" || state.paneOwner[wantRight] != 1 {
					t.Fatal("new file did not remain in the pane that initiated Open")
				}
			case "current clone":
				wantActive, wantRight = 1, loaded
			}
			if state.activePane != wantActive || state.paneOwner[loaded] != 0 || state.paneDocs != [2]*document.Document{loaded, wantRight} || len(state.space.Documents) != wantTabs || state.space.Current() != state.paneDocs[wantActive] {
				t.Fatalf("Open changed pane ownership or selected the wrong tab: pane=%d owner=%d current=%s", state.activePane, state.paneOwner[loaded], state.space.Current().Name)
			}
			if loaded.Editor.String() != value || loaded.Editor.Selection() != selection || !loaded.Editor.CanUndo() || !loaded.Dirty() || right.Editor.String() != "independent" {
				t.Fatal("Open changed an existing draft, selection, or Undo history")
			}
			wantFocus := editorID
			if wantActive == 1 {
				wantFocus = rightEditorID
			}
			if driver.Snapshot().Focused != wantFocus {
				t.Fatal("Open did not return focus to the selected editor")
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != string(original) {
				t.Fatalf("Open changed saved bytes: %x, %v", got, err)
			}
		})
	}
}
