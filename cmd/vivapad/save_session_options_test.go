package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/session"
)

func TestSaveSessionWorkspaceCheckboxControlsFileBrowserExport(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "first.txt")
	if err := os.WriteFile(file, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "workspace")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	if err := state.addFolderRoot(folder); err != nil {
		t.Fatal(err)
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Save Session...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.saveSessionOptionsOpen || !state.saveSessionIncludeWorkspace {
		t.Fatal("visible workspace did not default to checked")
	}
	clickID(t, driver, saveSessionWorkspaceID)
	if state.saveSessionIncludeWorkspace || !state.folderVisible {
		t.Fatal("checkbox changed visible workspace or failed to uncheck")
	}
	if err := os.Remove(folder); err != nil {
		t.Fatal(err)
	}
	without := filepath.Join(root, "without.xml")
	if err := driver.QueueSaveFile(without, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveSessionConfirmID)
	stored, err := session.LoadNPP(without)
	if err != nil || stored.FileBrowserIncluded || len(stored.Main) != 1 || stored.Main[0].Path != state.space.Current().Path || state.saveSessionOptionsOpen {
		t.Fatalf("unchecked session = %+v, %v, open=%v notice=%q", stored, err, state.saveSessionOptionsOpen, state.notice)
	}
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	state.command("Save Session...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.saveSessionIncludeWorkspace {
		t.Fatal("checkbox default did not reset to visible workspace")
	}
	with := filepath.Join(root, "with.xml")
	if err := driver.QueueSaveFile(with, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveSessionConfirmID)
	stored, err = session.LoadNPP(with)
	if err != nil || !stored.FileBrowserIncluded || len(stored.FolderRoots) != 1 || stored.FolderRoots[0].Path != state.folderRoots[0] {
		t.Fatalf("checked session = %+v, %v, notice=%q", stored, err, state.notice)
	}
}

func TestSaveSessionOptionsCancelAndDirtyPreflight(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "first.txt")
	if err := os.WriteFile(file, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Save Session...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.saveSessionOptionsOpen || state.saveSessionIncludeWorkspace {
		t.Fatal("hidden workspace did not default unchecked")
	}
	clickID(t, driver, saveSessionCancelID)
	if state.saveSessionOptionsOpen {
		t.Fatal("Cancel left options open")
	}
	state.command("Save Session...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveSessionConfirmID)
	if state.saveSessionOptionsOpen || len(state.space.Documents) != 1 || state.space.Current().Dirty() {
		t.Fatal("picker cancellation changed document")
	}
	state.space.Current().Editor.Insert("draft")
	state.command("Save Session...")
	if state.saveSessionOptionsOpen || !strings.Contains(state.notice, "modified") {
		t.Fatalf("dirty export opened picker: %q", state.notice)
	}
}

func TestSaveSessionInvalidWorkspaceFailsBeforePicker(t *testing.T) {
	root := t.TempDir()
	file, folder := filepath.Join(root, "first.txt"), filepath.Join(root, "workspace")
	if err := os.WriteFile(file, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	if err := state.addFolderRoot(folder); err != nil {
		t.Fatal(err)
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Save Session...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(folder); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "session.xml")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveSessionConfirmID)
	if !state.saveSessionOptionsOpen || state.saveSessionError == "" {
		t.Fatal("invalid workspace silently advanced to the file picker")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid workspace wrote session: %v", err)
	}
	clickID(t, driver, saveSessionWorkspaceID)
	clickID(t, driver, saveSessionConfirmID)
	stored, err := session.LoadNPP(path)
	if err != nil || stored.FileBrowserIncluded || len(stored.Main) != 1 || state.saveSessionOptionsOpen {
		t.Fatalf("recovered export = %+v, %v, notice=%q", stored, err, state.notice)
	}
}

func TestCaptureSaveSessionOptionsWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_SESSION_OPTION_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_SESSION_OPTION_SHOTS to capture session options")
	}
	root := t.TempDir()
	file := filepath.Join(root, "first.txt")
	if err := os.WriteFile(file, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	state.command("Save Session...")
	web, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer web.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := web.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		for _, hit := range web.Snapshot().Hits {
			if hit.ID == saveSessionWorkspaceID && (hit.Bounds.Y < 0 || hit.Bounds.Y+hit.Bounds.Height > float32(size.height)-statusHeight) {
				t.Fatalf("%s checkbox clipped: %+v", size.name, hit.Bounds)
			}
		}
		if problems := headless.CheckLayout(web.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s options layout: %+v", size.name, problems)
		}
		if err := web.Screenshot(filepath.Join(output, "save-session-options-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	if err := web.Driver().Key(ui.KeyEscape, 0); err != nil || state.saveSessionOptionsOpen {
		t.Fatalf("Escape left options open: %v", err)
	}
}
