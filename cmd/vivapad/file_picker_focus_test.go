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

func TestSaveAsChooserReturnsToActiveEditor(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		for _, pane := range []struct {
			name  string
			right bool
		}{{"single", false}, {"right", true}} {
			for _, result := range []string{"cancel", "save"} {
				t.Run(size.name+"/"+pane.name+"/"+result, func(t *testing.T) {
					root := t.TempDir()
					source := filepath.Join(root, "source.txt")
					target := filepath.Join(root, "target.txt")
					paths := []string{source}
					if pane.right {
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
					doc := left
					focus := editorID
					expectedPane := 0
					if pane.right {
						expectedPane = 1
						doc = state.space.Documents[1]
						state.splitEnabled, state.activePane = true, 1
						state.paneDocs = [2]*document.Document{left, doc}
						state.paneOwner[doc] = 1
						state.space.Select(1)
						focus = rightEditorID
					}
					oldPath := doc.Path
					driver, err := app.NewDriver(newConfig(state), nil)
					if err != nil {
						t.Fatal(err)
					}
					defer driver.Close()
					if err := driver.Resize(size.width, size.height); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, focus)
					doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
					if err := driver.Type("x"); err != nil || doc.Editor.String() != "basex" {
						t.Fatalf("initial edit = %q, %v", doc.Editor.String(), err)
					}
					doc.Editor.SetSelection(text.Range{Start: 5, End: 1})
					selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
					suggestedName := doc.Name
					if result == "cancel" {
						err = driver.QueueSaveFile("", app.ErrCanceled)
					} else {
						err = driver.QueueSaveFile(target, nil)
					}
					if err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					revealFileMenuItem(t, driver, 3)
					clickID(t, driver, menuItemScope.At(3))
					if request, ok := driver.LastSaveFileRequest(); !ok || request.Title != "Save As" || request.SuggestedName != suggestedName || request.DefaultExtension != "txt" {
						t.Fatalf("Save As picker request = %+v, present=%v", request, ok)
					}
					requireEditorCommandFocus(t, driver, focus)
					if state.activePane != expectedPane || state.space.Current() != doc || doc.Editor.String() != "basex" || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret {
						t.Fatal("Save As changed the active pane, text, selection or caret")
					}
					if result == "cancel" {
						if doc.Path != oldPath || !doc.Dirty() {
							t.Fatal("Cancel changed source path or dirty state")
						}
					} else {
						canonical, err := filepath.EvalSymlinks(target)
						if err != nil || doc.Path != canonical || doc.Dirty() {
							t.Fatalf("Save As path/dirty = %q/%v, canonical=%q, err=%v", doc.Path, doc.Dirty(), canonical, err)
						}
					}
					if disk, err := os.ReadFile(oldPath); err != nil || string(disk) != "base" {
						t.Fatalf("Save As changed source bytes: %q, %v", disk, err)
					}
					if result == "save" {
						if disk, err := os.ReadFile(target); err != nil || string(disk) != "basex" {
							t.Fatalf("Save As target bytes = %q, %v", disk, err)
						}
					}
					if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != "base" {
						t.Fatalf("first Undo after chooser = %q, handled=%v, err=%v", doc.Editor.String(), handled, err)
					}
					if pane.right && (left.Editor.String() != "base" || left.Dirty() || state.paneDocs[0] != left || state.paneDocs[1] != doc) {
						t.Fatal("Save As or Undo changed the inactive pane")
					}
					if _, err := os.Stat(sessionPath); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("Save As wrote an early session: %v", err)
					}
				})
			}
		}
	}
}

func TestToolbarAndShortcutChoosersReturnToEditor(t *testing.T) {
	for _, action := range []struct {
		name string
		open bool
		run  func(*testing.T, *app.Driver)
	}{
		{"toolbar-open", true, func(t *testing.T, driver *app.Driver) { clickID(t, driver, openID) }},
		{"shortcut-open", true, func(t *testing.T, driver *app.Driver) {
			if handled, err := driver.ShortcutChord(primary(), 'o'); err != nil || !handled {
				t.Fatalf("Ctrl+O = handled %v, %v", handled, err)
			}
		}},
		{"toolbar-save-untitled", false, func(t *testing.T, driver *app.Driver) { clickID(t, driver, saveID) }},
		{"shortcut-save-as", false, func(t *testing.T, driver *app.Driver) {
			if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 's'); err != nil || !handled {
				t.Fatalf("Ctrl+Shift+S = handled %v, %v", handled, err)
			}
		}},
	} {
		for _, result := range []string{"cancel", "success"} {
			t.Run(action.name+"/"+result, func(t *testing.T) {
				state, driver := startDriver(t)
				doc := state.space.Current()
				clickID(t, driver, editorID)
				if err := driver.Type("draft"); err != nil || doc.Editor.String() != "draft" {
					t.Fatalf("initial document = %q, %v", doc.Editor.String(), err)
				}
				target := filepath.Join(t.TempDir(), "target.txt")
				if action.open {
					if err := os.WriteFile(target, []byte("opened"), 0600); err != nil {
						t.Fatal(err)
					}
					var paths []string
					var err error
					if result == "success" {
						paths = []string{target}
					} else {
						err = app.ErrCanceled
					}
					if err := driver.QueueOpenFiles(paths, err); err != nil {
						t.Fatal(err)
					}
				} else {
					path, queueErr := "", error(app.ErrCanceled)
					if result == "success" {
						path, queueErr = target, nil
					}
					if err := driver.QueueSaveFile(path, queueErr); err != nil {
						t.Fatal(err)
					}
				}
				action.run(t, driver)
				if action.open {
					if request, ok := driver.LastOpenFilesRequest(); !ok || request.Title != "Open" || !request.AllowMultiple {
						t.Fatalf("Open request = %+v, present=%v", request, ok)
					}
				} else if request, ok := driver.LastSaveFileRequest(); !ok || request.Title != "Save As" {
					t.Fatalf("Save request = %+v, present=%v", request, ok)
				}
				requireEditorCommandFocus(t, driver, editorID)
				active := state.space.Current()
				before := "draft"
				if action.open && result == "success" {
					if active == doc || doc.Editor.String() != "draft" || !doc.Dirty() {
						t.Fatal("Open changed the original unsaved document")
					}
					before = "opened"
				} else if active != doc {
					t.Fatal("chooser changed the active document unexpectedly")
				}
				active.Editor.MoveTo(active.Editor.RuneCount(), false)
				if err := driver.Type("x"); err != nil || active.Editor.String() != before+"x" {
					t.Fatalf("first typing after chooser = %q, %v", active.Editor.String(), err)
				}
				if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || active.Editor.String() != before {
					t.Fatalf("first Undo after chooser = %q, handled=%v, %v", active.Editor.String(), handled, err)
				}
			})
		}
	}
}

func TestChooserShortcutDismissesOpenFileMenuAfterCancel(t *testing.T) {
	for _, action := range []struct {
		name string
		open bool
		key  rune
		mods gfx.Modifiers
	}{
		{"Open", true, 'o', primary()},
		{"Save As", false, 's', primary() | gfx.ModifierShift},
	} {
		t.Run(action.name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			clickID(t, driver, editorID)
			if err := driver.Type("draft"); err != nil {
				t.Fatal(err)
			}
			doc.Editor.BreakUndoGroup()
			if action.open {
				if err := driver.QueueOpenFiles(nil, app.ErrCanceled); err != nil {
					t.Fatal(err)
				}
			} else if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(0))
			if state.menuOpen != 0 {
				t.Fatal("File menu was not open before the shortcut")
			}
			if handled, err := driver.ShortcutChord(action.mods, action.key); err != nil || !handled {
				t.Fatalf("%s shortcut = handled %v, %v", action.name, handled, err)
			}
			if action.open {
				if _, ok := driver.LastOpenFilesRequest(); !ok {
					t.Fatal("Open shortcut did not invoke its picker")
				}
			} else if _, ok := driver.LastSaveFileRequest(); !ok {
				t.Fatal("Save As shortcut did not invoke its picker")
			}
			requireEditorCommandFocus(t, driver, editorID)
			if state.menuOpen != -1 || state.space.Current() != doc || doc.Editor.String() != "draft" || doc.Path != "" {
				t.Fatalf("Cancel left menu=%d, text=%q, path=%q", state.menuOpen, doc.Editor.String(), doc.Path)
			}
			if err := driver.Type("x"); err != nil || doc.Editor.String() != "draftx" {
				t.Fatalf("first typing after shortcut Cancel = %q, %v", doc.Editor.String(), err)
			}
			if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != "draft" {
				t.Fatalf("first Undo after shortcut Cancel = %q, handled=%v, %v", doc.Editor.String(), handled, err)
			}
		})
	}
}

func TestSaveAsShortcutFromMenuFocusesBeforeNextKey(t *testing.T) {
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
	if driver.Snapshot().Focused == editorID {
		t.Fatal("the File menu did not take focus before the chooser")
	}
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 's'); err != nil || !handled {
		t.Fatalf("Ctrl+Shift+S = handled %v, %v", handled, err)
	}
	if _, ok := driver.LastSaveFileRequest(); !ok || state.menuOpen != -1 || doc.Dirty() || state.space.Current() != doc {
		t.Fatal("Save As did not complete from the open File menu")
	}
	if disk, err := os.ReadFile(target); err != nil || string(disk) != "draft" {
		t.Fatalf("saved bytes = %q, %v", disk, err)
	}
	if got := driver.Snapshot().Focused; got != editorID || state.focusViewRequested {
		t.Fatalf("first post-chooser frame focus = %v, requested=%v, want ready editor", got, state.focusViewRequested)
	}
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != "" {
		t.Fatalf("first Undo after Save As = %q, handled=%v, %v", doc.Editor.String(), handled, err)
	}
}

func TestCanceledSaveAsFromDirtyCloseDoesNotStealDialogFocus(t *testing.T) {
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
	if _, ok := driver.LastSaveFileRequest(); !ok || state.pendingClose < 0 || !doc.Dirty() || doc.Editor.String() != "draft" {
		t.Fatal("canceled Save As dismissed the dirty close confirmation")
	}
	if state.focusViewRequested || driver.Snapshot().Focused == editorID {
		t.Fatalf("canceled Save As stole focus from the close dialog: requested=%v focused=%v", state.focusViewRequested, driver.Snapshot().Focused)
	}
}

func TestOpenChooserReturnsToActiveEditor(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		for _, pane := range []struct {
			name  string
			right bool
		}{{"single", false}, {"right", true}} {
			for _, result := range []string{"cancel", "open"} {
				t.Run(size.name+"/"+pane.name+"/"+result, func(t *testing.T) {
					root := t.TempDir()
					oldPath := filepath.Join(root, "source.txt")
					openedPath := filepath.Join(root, "opened.txt")
					paths := []string{oldPath}
					if pane.right {
						paths = append(paths, filepath.Join(root, "right.txt"))
					}
					for _, path := range paths {
						if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.WriteFile(openedPath, []byte("new\r\nfile\n"), 0600); err != nil {
						t.Fatal(err)
					}
					sessionPath := filepath.Join(root, "session.json")
					state := newStartupModel(paths, sessionPath)
					left := state.space.Documents[0]
					original := left
					focus := editorID
					expectedPane := 0
					if pane.right {
						expectedPane = 1
						original = state.space.Documents[1]
						state.splitEnabled, state.activePane = true, 1
						state.paneDocs = [2]*document.Document{left, original}
						state.paneOwner[original] = 1
						state.space.Select(1)
						focus = rightEditorID
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
					if err := driver.Type("x"); err != nil || original.Editor.String() != "basex" {
						t.Fatalf("initial edit = %q, %v", original.Editor.String(), err)
					}
					original.Editor.SetSelection(text.Range{Start: 5, End: 1})
					selection, caret := original.Editor.Selection(), original.Editor.Caret()
					if result == "cancel" {
						err = driver.QueueOpenFiles(nil, app.ErrCanceled)
					} else {
						err = driver.QueueOpenFiles([]string{openedPath}, nil)
					}
					if err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					revealFileMenuItem(t, driver, 1)
					clickID(t, driver, menuItemScope.At(1))
					if request, ok := driver.LastOpenFilesRequest(); !ok || request.Title != "Open" || !request.AllowMultiple {
						t.Fatalf("Open picker request = %+v, present=%v", request, ok)
					}
					requireEditorCommandFocus(t, driver, focus)
					if state.activePane != expectedPane || original.Editor.String() != "basex" || original.Editor.Selection() != selection || original.Editor.Caret() != caret || !original.Dirty() {
						t.Fatal("Open changed the original dirty document or its selection")
					}
					if result == "cancel" {
						if state.space.Current() != original || state.paneDocs[state.activePane] != original || len(state.space.Documents) != len(paths) {
							t.Fatal("Open Cancel changed document identity or active pane")
						}
						if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || original.Editor.String() != "base" {
							t.Fatalf("first Undo after Open Cancel = %q, handled=%v, err=%v", original.Editor.String(), handled, err)
						}
					} else {
						opened := state.space.Current()
						canonical, err := filepath.EvalSymlinks(openedPath)
						if err != nil || opened == original || opened.Path != canonical || opened.Editor.String() != "new\nfile\n" || state.paneDocs[state.activePane] != opened {
							t.Fatalf("Open result = %q, pane=%d, path=%q", opened.Editor.String(), state.activePane, opened.Path)
						}
						opened.Editor.MoveTo(opened.Editor.RuneCount(), false)
						if err := driver.Type("x"); err != nil || opened.Editor.String() != "new\nfile\nx" {
							t.Fatalf("first typing after Open = %q, %v", opened.Editor.String(), err)
						}
						if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || opened.Editor.String() != "new\nfile\n" {
							t.Fatalf("first Undo after Open = %q, handled=%v, err=%v", opened.Editor.String(), handled, err)
						}
					}
					if disk, err := os.ReadFile(original.Path); err != nil || string(disk) != "base" {
						t.Fatalf("Open changed original disk bytes: %q, %v", disk, err)
					}
					if disk, err := os.ReadFile(openedPath); err != nil || string(disk) != "new\r\nfile\n" {
						t.Fatalf("Open changed target disk bytes: %q, %v", disk, err)
					}
					if pane.right && (left.Editor.String() != "base" || left.Dirty() || state.paneDocs[0] != left) {
						t.Fatal("Open or subsequent editing changed the inactive pane")
					}
					if _, err := os.Stat(sessionPath); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("Open wrote an early session: %v", err)
					}
				})
			}
		}
	}
}
