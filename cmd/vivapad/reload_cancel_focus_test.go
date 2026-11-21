package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestDirtyReloadCancelRestoresActiveEditor(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		t.Run(size.name, func(t *testing.T) {
			for _, layout := range []struct {
				name  string
				split bool
				pane  int
				clone bool
			}{{"single", false, 0, false}, {"left", true, 0, false}, {"right", true, 1, false}, {"clone-right", true, 1, true}} {
				t.Run(layout.name, func(t *testing.T) {
					for _, cancel := range []string{"pointer", "escape"} {
						t.Run(cancel, func(t *testing.T) {
							root := t.TempDir()
							paths := []string{filepath.Join(root, "left.txt")}
							if layout.split && !layout.clone {
								paths = append(paths, filepath.Join(root, "right.txt"))
							}
							for _, path := range paths {
								if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
									t.Fatal(err)
								}
							}
							state := newStartupModel(paths, filepath.Join(root, "session.json"))
							left := state.space.Documents[0]
							right := left
							if len(paths) == 2 {
								right = state.space.Documents[1]
								state.paneOwner[right] = 1
							}
							state.splitEnabled, state.activePane = layout.split, layout.pane
							state.paneDocs = [2]*document.Document{left, right}
							doc := state.paneDocs[layout.pane]
							state.space.Select(state.documentIndex(doc))
							config := newConfig(state)
							driver, err := app.NewDriver(config, nil)
							if err != nil {
								t.Fatal(err)
							}
							defer driver.Close()
							if err := driver.Resize(size.width, size.height); err != nil {
								t.Fatal(err)
							}
							focus := editorID
							if layout.pane == 1 {
								focus = rightEditorID
							}
							clickID(t, driver, focus)
							doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
							if err := driver.Type("draft"); err != nil {
								t.Fatal(err)
							}
							doc.Editor.SetSelection(text.Range{Start: 4, End: 1})
							selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
							canUndo, canRedo, dirty := doc.Editor.CanUndo(), doc.Editor.CanRedo(), doc.Dirty()
							clickID(t, driver, menuScope.At(0))
							revealFileMenuItem(t, driver, 9)
							clickID(t, driver, menuItemScope.At(9))
							if state.pendingReload != state.documentIndex(doc) {
								t.Fatal("dirty reload bypassed confirmation")
							}
							if err := driver.Pump(); err != nil {
								t.Fatal(err)
							}
							if cancel == "pointer" {
								clickID(t, driver, reloadCancelID)
							} else if err := driver.Key(gfx.KeyEscape, 0); err != nil {
								t.Fatal(err)
							}
							requireEditorCommandFocus(t, driver, focus)
							if state.pendingReload >= 0 || state.space.Current() != doc || state.activePane != layout.pane ||
								doc.Editor.Selection() != selection || doc.Editor.Caret() != caret ||
								doc.Editor.CanUndo() != canUndo || doc.Editor.CanRedo() != canRedo || doc.Dirty() != dirty || doc.Editor.String() != "basedraft" {
								t.Fatal("Cancel changed the active document, selection, history or dirty state")
							}
							if err := driver.Type("tail"); err != nil || doc.Editor.String() != "btaildraft" {
								t.Fatalf("typing immediately after Reload Cancel = %q, %v", doc.Editor.String(), err)
							}
							if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != "basedraft" {
								t.Fatalf("Undo after Reload Cancel = %q, handled=%v, err=%v", doc.Editor.String(), handled, err)
							}
							for _, path := range paths {
								if value, err := os.ReadFile(path); err != nil || string(value) != "base" {
									t.Fatalf("Cancel wrote %q: %q, %v", path, value, err)
								}
							}
						})
					}
				})
			}
		})
	}
}
