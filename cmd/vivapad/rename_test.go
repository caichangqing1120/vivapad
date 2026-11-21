package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/session"
)

func TestRenameSavedDirtyFileMovesDiskWithoutSavingBuffer(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldPath, newPath := filepath.Join(root, "old.txt"), filepath.Join(root, "new.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{oldPath}, filepath.Join(root, "data", "session.json"))
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert(" local")
	state.command("Toggle Bookmark")
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueSaveFile(newPath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	if data, err := os.ReadFile(newPath); err != nil || string(data) != "old" {
		t.Fatalf("renamed disk file = %q, %v", data, err)
	}
	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old file still present: %v", err)
	}
	if doc.Path != newPath || doc.Editor.String() != "old local" || !doc.Dirty() || len(state.bookmarkLines(doc)) != 1 ||
		len(state.recentEntries) == 0 || state.recentEntries[0].Path != newPath {
		t.Fatalf("rename lost buffer state: path=%q dirty=%v marks=%v recent=%+v", doc.Path, doc.Dirty(), state.bookmarkLines(doc), state.recentEntries)
	}
	for _, entry := range state.recentEntries {
		if entry.Path == oldPath {
			t.Fatal("old path stayed in recent files")
		}
	}
}

func TestRenameChooserReturnsToEditor(t *testing.T) {
	for _, trial := range []struct {
		name            string
		cancelPicker    bool
		replace, accept bool
	}{
		{name: "picker-cancel", cancelPicker: true},
		{name: "new-destination", accept: true},
		{name: "replace-cancel", replace: true},
		{name: "replace-confirm", replace: true, accept: true},
	} {
		t.Run(trial.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			leftPath, source, target := filepath.Join(root, "left.txt"), filepath.Join(root, "old.txt"), filepath.Join(root, "new.txt")
			files := map[string]string{leftPath: "left", source: "base"}
			if trial.replace {
				files[target] = "previous"
			}
			for path, value := range files {
				if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			state := newModelWithFiles([]string{leftPath, source})
			left, doc := state.space.Documents[0], state.space.Documents[1]
			state.splitEnabled, state.activePane = true, 1
			state.paneDocs = [2]*document.Document{left, doc}
			state.paneOwner[doc] = 1
			state.space.Select(1)
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, rightEditorID)
			doc.Editor.MoveTo(4, false)
			if err := driver.Type("x"); err != nil || doc.Editor.String() != "basex" {
				t.Fatalf("initial edit = %q, %v", doc.Editor.String(), err)
			}
			var response error
			if trial.cancelPicker {
				response = app.ErrCanceled
			}
			if err := driver.QueueSaveFile(target, response); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(0))
			revealFileMenuItem(t, driver, 14)
			clickID(t, driver, menuItemScope.At(14))
			if trial.replace {
				if state.renamePlan == nil || state.focusViewRequested || driver.Snapshot().Focused == rightEditorID {
					t.Fatal("editor focus bypassed the overwrite confirmation")
				}
				if trial.accept {
					clickID(t, driver, renameConfirmID)
				} else {
					clickID(t, driver, renameCancelID)
				}
			}
			wantPath := source
			if trial.accept {
				wantPath = target
				files[target] = "base"
				delete(files, source)
				if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("accepted rename left source in place: %v", err)
				}
			} else if !trial.replace {
				if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("canceled chooser created destination: %v", err)
				}
			}
			if doc.Path != wantPath || state.activePane != 1 || state.space.Current() != doc || !doc.Dirty() || doc.Editor.String() != "basex" || doc.Editor.Caret() != 5 {
				t.Fatal("rename changed the active editor, dirty buffer or caret")
			}
			requireEditorCommandFocus(t, driver, rightEditorID)
			if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != "base" || left.Editor.String() != "left" {
				t.Fatalf("first Undo = %q, handled=%v, %v", doc.Editor.String(), handled, err)
			}
			for path, want := range files {
				if bytes, err := os.ReadFile(path); err != nil || string(bytes) != want {
					t.Fatalf("%s = %q, %v; want %q", path, bytes, err, want)
				}
			}
		})
	}
}

func TestRenameExistingDestinationRequiresConfirmationAndRechecks(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldPath, target := filepath.Join(root, "old.txt"), filepath.Join(root, "target.txt")
	for path, content := range map[string]string{oldPath: "source", target: "previous"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{oldPath})
	doc := state.space.Current()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	choose := func() {
		if err := driver.QueueSaveFile(target, nil); err != nil {
			t.Fatal(err)
		}
		clickID(t, driver, menuScope.At(0))
		clickID(t, driver, menuItemScope.At(14))
	}
	choose()
	if state.renamePlan == nil || doc.Path != oldPath {
		t.Fatal("existing destination was replaced before confirmation")
	}
	clickID(t, driver, renameCancelID)
	choose()
	if err := os.WriteFile(target, []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, renameConfirmID)
	if doc.Path != oldPath || state.renamePlan == nil || !strings.Contains(state.notice, "destination changed") {
		t.Fatalf("stale confirmation changed binding: path=%q notice=%q", doc.Path, state.notice)
	}
	clickID(t, driver, renameCancelID)
	choose()
	clickID(t, driver, renameConfirmID)
	if doc.Path != target || state.renamePlan != nil {
		t.Fatalf("confirmed rename = path %q plan %+v", doc.Path, state.renamePlan)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "source" {
		t.Fatalf("confirmed replacement = %q, %v", data, err)
	}
}

func TestRenameCanceledOrOpenTabTargetDoesNotChangeFiles(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldPath, otherPath := filepath.Join(root, "old.txt"), filepath.Join(root, "other.txt")
	for path, content := range map[string]string{oldPath: "old", otherPath: "other"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{oldPath, otherPath})
	state.space.Select(0)
	doc := state.space.Current()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	if err := driver.QueueSaveFile(otherPath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	if doc.Path != oldPath || state.renamePlan != nil || !strings.Contains(state.notice, "already open") {
		t.Fatalf("open tab collision = path %q, notice %q", doc.Path, state.notice)
	}
	if data, err := os.ReadFile(otherPath); err != nil || string(data) != "other" {
		t.Fatalf("other tab overwritten: %q, %v", data, err)
	}
}

func TestRenameRejectsHardLinkToAnotherOpenTab(t *testing.T) {
	root := t.TempDir()
	source, other, alias := filepath.Join(root, "source.txt"), filepath.Join(root, "other.txt"), filepath.Join(root, "alias.txt")
	for path, data := range map[string]string{source: "original", other: "second"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Link(other, alias); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	state := newModelWithFiles([]string{source, other})
	state.space.Select(0)
	doc, originalPath := state.space.Current(), state.space.Current().Path
	doc.Editor.Insert("draft ")
	value := doc.Editor.String()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.QueueSaveFile(alias, nil); err != nil {
		t.Fatal(err)
	}
	state.requestRename()
	if state.renamePlan != nil || !strings.Contains(state.notice, "already open in another tab") {
		t.Fatalf("alias reached overwrite confirmation: plan=%v notice=%q", state.renamePlan != nil, state.notice)
	}
	if doc.Path != originalPath || !doc.Dirty() || doc.Editor.String() != value {
		t.Fatal("rename refusal changed the open draft")
	}
	for path, expected := range map[string]string{source: "original", other: "second", alias: "second"} {
		if got, err := os.ReadFile(path); err != nil || string(got) != expected {
			t.Fatalf("rename refusal changed %s: %q, %v", path, got, err)
		}
	}
}

func TestRenameUntitledTabValidatesNameWithoutWritingDisk(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("draft")
	clickID(t, driver, newID)
	other := state.space.Current()
	state.space.Select(0)
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	if state.renameDoc != doc {
		t.Fatal("untitled rename dialog did not open")
	}
	state.renameField.SetSelection(text.Range{Start: 0, End: state.renameField.RuneCount()})
	state.renameField.Insert(other.Name)
	clickID(t, driver, renameApplyID)
	if doc.Name != "new 1" || state.renameDoc == nil || !strings.Contains(state.notice, "already in use") {
		t.Fatalf("duplicate name accepted: %q, notice %q", doc.Name, state.notice)
	}
	state.renameField.SetSelection(text.Range{Start: 0, End: state.renameField.RuneCount()})
	state.renameField.Insert("notes")
	clickID(t, driver, renameApplyID)
	if doc.Name != "notes" || doc.Path != "" || doc.Editor.String() != "draft" || !doc.Dirty() || state.renameDoc != nil {
		t.Fatalf("untitled rename changed content or path: %+v", doc)
	}
}

func TestRenameUntitledDialogAcceptsTypingAndEscapeBlocksUnderlyingCommands(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	if _, err := driver.ShortcutChord(primary(), 'n'); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 1 || state.space.Current() != doc {
		t.Fatal("new-document shortcut passed through Rename dialog")
	}
	clickID(t, driver, renameFieldID)
	if _, err := driver.ShortcutChord(primary(), 'a'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("meeting-notes"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Name != "meeting-notes" || state.renameDoc != nil {
		t.Fatalf("typed rename = %q, dialog=%v", doc.Name, state.renameDoc != nil)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Name != "meeting-notes" || state.renameDoc != nil {
		t.Fatalf("Escape changed tab name: %q", doc.Name)
	}
}

func TestRenameUntitledRejectsPathLikeNameWithoutClosingDialog(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	state.renameField.SetSelection(text.Range{Start: 0, End: state.renameField.RuneCount()})
	state.renameField.Insert("folder/notes")
	clickID(t, driver, renameApplyID)
	if state.renameDoc != doc || doc.Name != "new 1" || !strings.Contains(state.notice, "valid tab name") {
		t.Fatalf("path-like tab rename = name %q, notice %q", doc.Name, state.notice)
	}
}

func TestRenameTargetSymlinkToOpenTabIsRejected(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldPath, otherPath, alias := filepath.Join(root, "old.txt"), filepath.Join(root, "other.txt"), filepath.Join(root, "alias.txt")
	for path, content := range map[string]string{oldPath: "old", otherPath: "other"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(otherPath, alias); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{oldPath, otherPath})
	state.space.Select(0)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueSaveFile(alias, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	if state.space.Current().Path != oldPath || state.renamePlan != nil || !strings.Contains(state.notice, "not a regular file") {
		t.Fatalf("symlink alias accepted: path=%q notice=%q", state.space.Current().Path, state.notice)
	}
}

func TestRenameConfirmationRemainsBoundToOriginalDocument(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldPath, otherPath, target := filepath.Join(root, "old.txt"), filepath.Join(root, "other.txt"), filepath.Join(root, "target.txt")
	for path, content := range map[string]string{oldPath: "old", otherPath: "other", target: "previous"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{oldPath, otherPath})
	first, second := state.space.Documents[0], state.space.Documents[1]
	state.space.Select(0)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueSaveFile(target, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	state.space.Select(1)
	clickID(t, driver, renameConfirmID)
	if first.Path != target || second.Path != otherPath || len(state.recentEntries) == 0 || state.recentEntries[0].Path != target {
		t.Fatalf("confirmation changed wrong document: first=%q second=%q recent=%+v", first.Path, second.Path, state.recentEntries)
	}
}

func TestRenamePersistsNewPathInNextSession(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldPath, target := filepath.Join(root, "old.txt"), filepath.Join(root, "renamed.txt")
	if err := os.WriteFile(oldPath, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(root, "data", "session.json")
	state := newStartupModel([]string{oldPath}, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueSaveFile(target, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	stored, err := session.Load(sessionPath)
	if err != nil || !allowed || len(stored.Paths) != 1 || stored.Paths[0] != target || stored.Active != target {
		t.Fatalf("renamed session = %+v, allowed=%v, err=%v", stored, allowed, err)
	}
	if restored := newStartupModel(nil, sessionPath).space.Current(); restored.Path != target || restored.Editor.String() != "saved" {
		t.Fatalf("restored renamed tab = %+v", restored)
	}
}

func TestRenameDialogBlocksWindowCloseUntilCanceled(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	clickID(t, driver, newID)
	if len(state.space.Documents) != 1 {
		t.Fatal("toolbar click passed through Rename dialog")
	}
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	if allowed || state.renameDoc == nil || !strings.Contains(state.notice, "Rename") {
		t.Fatalf("close passed rename dialog: allowed=%v notice=%q", allowed, state.notice)
	}
	clickID(t, driver, renameCancelID)
	newConfig(state).OnCloseRequest(func() { allowed = true })
	if !allowed {
		t.Fatal("clean window could not close after canceling Rename")
	}
}

func TestRenameRefusesPrivateVivaPadDataDestination(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(root, "source.txt")
	if err := os.WriteFile(oldPath, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	privateDir := filepath.Join(root, "data")
	if err := os.Mkdir(privateDir, 0700); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(privateDir, "session.json")
	state := newStartupModel([]string{oldPath}, privatePath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueSaveFile(privatePath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(14))
	if state.space.Current().Path != oldPath || !strings.Contains(state.notice, "internal VivaPad") {
		t.Fatalf("private rename destination accepted: path=%q notice=%q", state.space.Current().Path, state.notice)
	}
	if data, err := os.ReadFile(oldPath); err != nil || string(data) != "source" {
		t.Fatalf("private rename moved source: %q, %v", data, err)
	}
}
