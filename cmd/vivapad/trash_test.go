package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/recent"
)

func TestTrashConfirmsDirtyCloneThenClosesBothViews(t *testing.T) {
	root := t.TempDir()
	file, moved := filepath.Join(root, "first.txt"), filepath.Join(root, "recycled.txt")
	if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	doc := state.space.Current()
	doc.Editor.Insert("draft ")
	state.toggleSplit()
	state.paneDocs[1] = doc
	state.activePane = 1
	state.rememberRecent(doc)
	state.recordClosed(doc)
	called := false
	state.trashPath = func(path string) error {
		called = true
		if path != doc.Path {
			t.Fatalf("trash path = %q, want %q", path, doc.Path)
		}
		return os.Rename(path, moved)
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Move to Recycle Bin")
	if state.pendingTrash == nil || called {
		t.Fatal("trash bypassed the confirmation")
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "saved" {
		t.Fatalf("confirmation touched disk: %q, %v", data, err)
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, trashConfirmID)
	if !called || state.pendingTrash != nil || state.documentIndex(doc) >= 0 || state.paneDocs[0] == doc || state.paneDocs[1] == doc {
		t.Fatalf("recycled clone remains: called=%v pending=%v panes=%+v", called, state.pendingTrash, state.paneDocs)
	}
	if data, err := os.ReadFile(moved); err != nil || string(data) != "saved" {
		t.Fatalf("saved bytes not moved: %q, %v", data, err)
	}
	for _, entry := range append(state.recentEntries, state.closedHistory...) {
		if entry.Path == doc.Path {
			t.Fatalf("trashed path remained in history: %+v", entry)
		}
	}
}

func TestTrashCancelStaleAndBackendFailurePreserveDocument(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	doc := state.space.Current()
	called := false
	state.trashPath = func(string) error { called = true; return errors.New("trash unavailable") }
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Move to Recycle Bin")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, trashCancelID)
	if state.pendingTrash != nil || called || state.space.Current() != doc {
		t.Fatal("cancel moved or closed the document")
	}
	state.command("Move to Recycle Bin")
	if err := os.WriteFile(file, []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, trashConfirmID)
	if called || state.pendingTrash == nil || state.space.Current() != doc || !strings.Contains(state.notice, "changed on disk") {
		t.Fatalf("stale file was trashed: called=%v notice=%q", called, state.notice)
	}
	clickID(t, driver, trashCancelID)
	if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state.command("Move to Recycle Bin")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, trashConfirmID)
	if !called || state.pendingTrash == nil || state.space.Current() != doc || !strings.Contains(state.notice, "trash unavailable") {
		t.Fatalf("backend failure closed document: called=%v notice=%q", called, state.notice)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "saved" {
		t.Fatalf("backend failure touched disk: %q, %v", data, err)
	}
}

func TestTrashRejectsUntitledAndPrivateDataAndFitsCompactMenu(t *testing.T) {
	state := newModel()
	state.command("Move to Recycle Bin")
	if state.pendingTrash != nil || !strings.Contains(state.notice, "Save") {
		t.Fatalf("untitled trash = %+v, %q", state.pendingTrash, state.notice)
	}
	root := t.TempDir()
	private := filepath.Join(root, "session.json")
	if err := os.WriteFile(private, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	state = newModelWithFiles([]string{private})
	state.sessionPath = state.space.Current().Path
	state.command("Move to Recycle Bin")
	if state.pendingTrash != nil || !strings.Contains(state.notice, "internal") {
		t.Fatalf("private data trash = %+v, %q", state.pendingTrash, state.notice)
	}
	driver, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver.Driver(), menuScope.At(0))
	if hit := revealFileMenuItem(t, driver.Driver(), 20); hit.Y+hit.Height > 400-statusHeight {
		t.Fatalf("trash menu row clipped: %+v", hit)
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) > 0 {
		t.Fatalf("compact File menu layout: %+v", problems)
	}
	state.sessionPath = ""
	clickID(t, driver.Driver(), menuItemScope.At(20))
	if state.pendingTrash == nil {
		t.Fatal("File menu did not open trash confirmation")
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) > 0 {
		t.Fatalf("compact trash confirmation layout: %+v", problems)
	}
}

func TestTrashUpdatesClosedHistoryEvenWhenRecentSaveFails(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	state.closedPath = filepath.Join(root, "closed.json")
	state.recordClosed(state.space.Current())
	state.recentPath = filepath.Join(root, "directory-instead-of-recent-file")
	if err := os.Mkdir(state.recentPath, 0700); err != nil {
		t.Fatal(err)
	}
	state.trashPath = func(path string) error { return os.Rename(path, filepath.Join(root, "moved.txt")) }
	state.command("Move to Recycle Bin")
	state.confirmTrash()
	stored, err := recent.Load(state.closedPath)
	if err != nil || len(stored) != 0 {
		t.Fatalf("closed history retained deleted path: %+v, %v", stored, err)
	}
	if !strings.Contains(state.notice, "recent history") {
		t.Fatalf("recent persistence failure was hidden: %q", state.notice)
	}
}

func TestCaptureTrashConfirmationWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_TRASH_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_TRASH_SHOTS to capture trash confirmation")
	}
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.space.Current().Editor.Insert("draft ")
	web, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer web.Close()
	if err := web.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, web.Driver(), menuScope.At(0))
	revealFileMenuItem(t, web.Driver(), 20)
	if problems := headless.CheckLayout(web.Snapshot()); len(problems) > 0 {
		t.Fatalf("compact File menu layout: %+v", problems)
	}
	if err := web.Screenshot(filepath.Join(output, "trash-file-menu-compact.png")); err != nil {
		t.Fatal(err)
	}
	clickID(t, web.Driver(), menuItemScope.At(20))
	if state.pendingTrash == nil {
		t.Fatal("File command did not open confirmation")
	}
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := web.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(web.Snapshot()); len(problems) > 0 {
			t.Fatalf("%s confirmation layout: %+v", size.name, problems)
		}
		if err := web.Screenshot(filepath.Join(output, "trash-confirm-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	if err := web.Driver().Key(ui.KeyEscape, 0); err != nil || state.pendingTrash != nil {
		t.Fatalf("Escape did not cancel trash: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "saved" {
		t.Fatalf("Escape touched source: %q, %v", data, err)
	}
}
