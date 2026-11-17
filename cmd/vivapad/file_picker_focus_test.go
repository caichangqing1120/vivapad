package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func TestNativeFileChooserReturnsFocusToOwningEditor(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		for _, right := range []bool{false, true} {
			for _, action := range []string{"open-cancel", "open-success", "save-cancel", "save-success"} {
				t.Run(size.name+"/"+action+map[bool]string{false: "/main", true: "/right"}[right], func(t *testing.T) {
					root := t.TempDir()
					sourcePath, targetPath := filepath.Join(root, "source.txt"), filepath.Join(root, "target.txt")
					for _, path := range []string{sourcePath, targetPath} {
						if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					state := newModelWithFiles([]string{sourcePath})
					original := state.space.Current()
					focus := editorID
					if right {
						other := state.space.NewDocument()
						other.Editor.Insert("right")
						state.splitEnabled, state.activePane = true, 1
						state.paneDocs = [2]*document.Document{original, other}
						state.paneOwner[other] = 1
						state.space.Select(1)
						original, focus = other, rightEditorID
					}
					driver, err := app.NewDriver(newConfig(state), nil)
					if err != nil {
						t.Fatal(err)
					}
					defer driver.Close()
					if err := driver.Resize(size.width, size.height); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, focus)
					original.Editor.MoveTo(original.Editor.RuneCount(), false)
					original.Editor.Insert("x")
					original.Editor.SetSelection(text.Range{Start: 1, End: 0})
					before, selection := original.Editor.String(), original.Editor.Selection()
					savedPath := filepath.Join(root, "saved.txt")
					if action == "open-success" {
						err = driver.QueueOpenFiles([]string{targetPath}, nil)
					} else if action == "open-cancel" {
						err = driver.QueueOpenFiles(nil, app.ErrCanceled)
					} else if action == "save-success" {
						err = driver.QueueSaveFile(savedPath, nil)
					} else {
						err = driver.QueueSaveFile("", app.ErrCanceled)
					}
					if err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					item := 1
					if action[:4] == "save" {
						item = 3
					}
					revealFileMenuItem(t, driver, item)
					clickID(t, driver, menuItemScope.At(item))
					requireEditorCommandFocus(t, driver, focus)
					if original.Editor.String() != before || original.Editor.Selection() != selection || state.activePane != map[bool]int{false: 0, true: 1}[right] {
						t.Fatal("chooser changed source editor or owning pane")
					}
					if action == "open-success" {
						if state.space.Current() == original || state.space.Current().Editor.String() != "base" {
							t.Fatal("Open did not activate target in the same pane")
						}
					} else if state.space.Current() != original {
						t.Fatal("chooser changed the source document")
					}
					if action == "save-success" {
						canonical, err := filepath.EvalSymlinks(savedPath)
						if err != nil || original.Path != canonical || original.Dirty() {
							t.Fatalf("Save As path/dirty = %q/%v, canonical=%q err=%v", original.Path, original.Dirty(), canonical, err)
						}
						if disk, err := os.ReadFile(savedPath); err != nil || string(disk) != before {
							t.Fatalf("Save As bytes = %q, %v", disk, err)
						}
					} else if !original.Dirty() {
						t.Fatal("chooser discarded the unsaved edit")
					}
					if disk, err := os.ReadFile(sourcePath); err != nil || string(disk) != "base" {
						t.Fatalf("source disk changed: %q, %v", disk, err)
					}
					if action != "open-success" {
						if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || original.Editor.String() != before[:len(before)-1] {
							t.Fatalf("first Undo after chooser: handled=%v text=%q err=%v", handled, original.Editor.String(), err)
						}
					}
				})
			}
		}
	}
}

func TestSaveAsShortcutFromFileMenuFocusesBeforeFirstUndo(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	clickID(t, driver, editorID)
	if err := driver.Type("draft"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "saved.txt")
	if err := driver.QueueSaveFile(target, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	if handled, err := driver.ShortcutChord(primary()|ui.ModifierShift, 's'); err != nil || !handled {
		t.Fatalf("Save As shortcut: handled=%v err=%v", handled, err)
	}
	if state.menuOpen != -1 || driver.Snapshot().Focused != editorID || state.focusViewRequested || doc.Dirty() {
		t.Fatalf("first frame: menu=%d focus=%v requested=%v dirty=%v", state.menuOpen, driver.Snapshot().Focused, state.focusViewRequested, doc.Dirty())
	}
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != "" {
		t.Fatalf("first Undo: handled=%v err=%v text=%q", handled, err, doc.Editor.String())
	}
}

func TestOpenShortcutFromFileMenuAndToolbarReturnFocus(t *testing.T) {
	for _, entry := range []string{"menu-shortcut", "toolbar"} {
		for _, outcome := range []string{"cancel", "open"} {
			t.Run(entry+"/"+outcome, func(t *testing.T) {
				state, driver := startDriver(t)
				source := state.space.Current()
				clickID(t, driver, editorID)
				if err := driver.Type("draft"); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "opened.txt")
				if err := os.WriteFile(path, []byte("opened"), 0600); err != nil {
					t.Fatal(err)
				}
				if outcome == "open" {
					if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
						t.Fatal(err)
					}
				} else if err := driver.QueueOpenFiles(nil, app.ErrCanceled); err != nil {
					t.Fatal(err)
				}
				if entry == "menu-shortcut" {
					clickID(t, driver, menuScope.At(0))
					if handled, err := driver.ShortcutChord(primary(), 'o'); err != nil || !handled {
						t.Fatalf("Open shortcut: handled=%v err=%v", handled, err)
					}
				} else {
					clickID(t, driver, openID)
				}
				requireEditorCommandFocus(t, driver, editorID)
				active := state.space.Current()
				if outcome == "cancel" && active != source || outcome == "open" && (active == source || active.Editor.String() != "opened") {
					t.Fatal("Open returned to the wrong document")
				}
				active.Editor.MoveTo(active.Editor.RuneCount(), false)
				before := active.Editor.String()
				if err := driver.Type("x"); err != nil || active.Editor.String() != before+"x" {
					t.Fatalf("first typing after Open = %q, %v", active.Editor.String(), err)
				}
			})
		}
	}
}

func TestCanceledSaveAsFromDirtyCloseKeepsDialogFocus(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("draft")
	state.command("Close")
	if state.pendingClose < 0 {
		t.Fatal("dirty close did not open its confirmation")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveCloseID)
	if state.pendingClose < 0 || state.focusViewRequested || driver.Snapshot().Focused == editorID || !doc.Dirty() {
		t.Fatalf("Cancel stole close-dialog focus: pending=%d requested=%v focused=%v", state.pendingClose, state.focusViewRequested, driver.Snapshot().Focused)
	}
}
