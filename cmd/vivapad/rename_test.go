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
