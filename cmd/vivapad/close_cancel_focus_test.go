package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestCloseCancelRestoresActiveEditor(t *testing.T) {
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
			}{{"single", false, 0, false}, {"left", true, 0, false}, {"right", true, 1, false}, {"clone-left", true, 0, true}, {"clone-right", true, 1, true}} {
				t.Run(layout.name, func(t *testing.T) {
					for _, cancel := range []string{"escape", "pointer"} {
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
							sessionPath := filepath.Join(root, "session.json")
							state := newStartupModel(paths, sessionPath)
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
							doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
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
							if err := driver.Type("draft"); err != nil {
								t.Fatal(err)
							}
							if doc.Editor.String() != "basedraft" {
								t.Fatalf("typing fixture = %q", doc.Editor.String())
							}
							doc.Editor.SetSelection(text.Range{Start: 4, End: 1})
							selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
							canUndo, canRedo, dirty := doc.Editor.CanUndo(), doc.Editor.CanRedo(), doc.Dirty()
							closed := 0
							config.OnCloseRequest(func() { closed++ })
							if closed != 0 || state.pendingClose != state.documentIndex(doc) {
								t.Fatal("dirty window close bypassed its confirmation")
							}
							if err := driver.Pump(); err != nil {
								t.Fatal(err)
							}
							if cancel == "pointer" {
								clickID(t, driver, cancelCloseID)
							} else if err := driver.Key(gfx.KeyEscape, 0); err != nil {
								t.Fatal(err)
							}
							if state.pendingClose >= 0 || closed != 0 || state.allowClose != nil || state.space.Current() != doc || state.activePane != layout.pane || driver.Snapshot().Focused != focus {
								t.Fatalf("Cancel focus handoff: pending=%d closed=%d pane=%d focus=%v; want pane=%d focus=%v", state.pendingClose, closed, state.activePane, driver.Snapshot().Focused, layout.pane, focus)
							}
							if doc.Editor.String() != "basedraft" || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Editor.CanUndo() != canUndo || doc.Editor.CanRedo() != canRedo || doc.Dirty() != dirty {
								t.Fatal("Cancel changed text, directional selection, history or dirty state")
							}
							if err := driver.Type("tail"); err != nil || doc.Editor.String() != "btaildraft" {
								t.Fatalf("typing immediately after Cancel = %q, %v", doc.Editor.String(), err)
							}
							for _, want := range []string{"basedraft", "base"} {
								if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != want {
									t.Fatalf("Undo after Cancel = %q; want %q, handled=%v err=%v", doc.Editor.String(), want, handled, err)
								}
							}
							for _, want := range []string{"basedraft", "btaildraft"} {
								if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'z'); err != nil || !handled || doc.Editor.String() != want {
									t.Fatalf("Redo after Cancel = %q; want %q, handled=%v err=%v", doc.Editor.String(), want, handled, err)
								}
							}
							for _, other := range state.space.Documents {
								if other != doc && (other.Editor.String() != "base" || other.Dirty() || other.Editor.CanUndo()) {
									t.Fatal("Cancel or further typing changed the inactive document")
								}
							}
							for _, path := range paths {
								if data, err := os.ReadFile(path); err != nil || string(data) != "base" {
									t.Fatalf("Cancel or typing saved %q: %q, %v", path, data, err)
								}
							}
							if _, err := os.Stat(sessionPath); !errors.Is(err, os.ErrNotExist) {
								t.Fatalf("canceled close persisted its session: %v", err)
							}
						})
					}
				})
			}
		})
	}
}

func TestCloseCancelBatchKeepsActiveEditor(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		t.Run(size.name, func(t *testing.T) {
			for _, cancel := range []string{"escape", "pointer"} {
				t.Run(cancel, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "clean.txt")
					if err := os.WriteFile(path, []byte("clean"), 0600); err != nil {
						t.Fatal(err)
					}
					state := newStartupModel([]string{path}, filepath.Join(root, "session.json"))
					clean := state.space.Current()
					pending := state.space.NewDocument()
					pending.Editor.Insert("pending")
					active := state.space.NewDocument()
					active.Editor.Insert("active")
					active.Editor.BreakUndoGroup()
					state.showInActivePane(active)
					config := newConfig(state)
					driver, err := app.NewDriver(config, nil)
					if err != nil {
						t.Fatal(err)
					}
					defer driver.Close()
					if err := driver.Resize(size.width, size.height); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, editorID)
					state.startBatchClose(nil)
					closed := 0
					config.OnCloseRequest(func() { closed++ })
					if !state.batchActive || state.pendingClose != state.documentIndex(pending) || state.documentIndex(clean) >= 0 || state.batchClosed != 1 {
						t.Fatal("partial batch did not pause at its dirty document")
					}
					if err := driver.Pump(); err != nil {
						t.Fatal(err)
					}
					if cancel == "pointer" {
						clickID(t, driver, cancelCloseID)
					} else if err := driver.Key(gfx.KeyEscape, 0); err != nil {
						t.Fatal(err)
					}
					if state.batchActive || state.pendingClose >= 0 || closed != 0 || state.space.Current() != active || driver.Snapshot().Focused != editorID || len(state.space.Documents) != 2 {
						t.Fatal("batch Cancel did not return to the active editor")
					}
					if err := driver.Type("tail"); err != nil || active.Editor.String() != "activetail" || pending.Editor.String() != "pending" || !pending.Dirty() {
						t.Fatalf("typing after batch Cancel = %q, pending=%q, err=%v", active.Editor.String(), pending.Editor.String(), err)
					}
					if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || active.Editor.String() != "active" {
						t.Fatalf("Undo after batch Cancel = %q, %v", active.Editor.String(), err)
					}
				})
			}
		})
	}
}

func TestCloseCancelPendingFocusYieldsToNewInput(t *testing.T) {
	for _, target := range []struct {
		name, command string
		focus         gfx.ID
		value         func(*model) string
	}{
		{"column", "Column Editor...", columnTextID, func(state *model) string { return state.columnText.String() }},
		{"mark", "Mark...", queryID, func(state *model) string { return state.query.String() }},
	} {
		t.Run(target.name, func(t *testing.T) {
			state, driver := startDriver(t)
			clickID(t, driver, editorID)
			if err := driver.Type("draft"); err != nil {
				t.Fatal(err)
			}
			newConfig(state).OnCloseRequest(func() { t.Fatal("dirty close was accepted") })
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			state.cancelClose()
			state.command(target.command)
			state.invalidator.Invalidate()
			requireEditorCommandFocus(t, driver, target.focus)
			if err := driver.Type("target"); err != nil || target.value(state) != "target" || state.space.Current().Editor.String() != "draft" {
				t.Fatalf("new input did not keep focus after Cancel: value=%q, err=%v", target.value(state), err)
			}
			requireEditorCommandFocus(t, driver, target.focus)
		})
	}
}
