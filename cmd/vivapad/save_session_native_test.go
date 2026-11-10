package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/session"
)

func TestSaveSessionNativeCheckboxControlsWorkspaceExport(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "first.txt")
	folder := filepath.Join(root, "workspace")
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
	state.saveSessionPicker = app.SaveFileWithCheckbox
	without := filepath.Join(root, "without.xml")
	if err := driver.QueueSaveFileWithCheckbox(without, false, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Save Session...")
	if state.saveSessionOptionsOpen {
		t.Fatal("native picker opened the fallback option dialog")
	}
	if _, checkbox, ok := driver.LastSaveFileCheckboxRequest(); !ok || checkbox.Label != "Save Folder as Workspace" || !checkbox.Checked {
		t.Fatalf("native checkbox request = %+v, %v", checkbox, ok)
	}
	stored, err := session.LoadNPP(without)
	if err != nil || stored.FileBrowserIncluded {
		t.Fatalf("unchecked native save = %+v, %v", stored, err)
	}
	with := filepath.Join(root, "with.xml")
	if err := driver.QueueSaveFileWithCheckbox(with, true, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Save Session...")
	stored, err = session.LoadNPP(with)
	if err != nil || !stored.FileBrowserIncluded || len(stored.FolderRoots) != 1 || stored.FolderRoots[0].Path != state.folderRoots[0] {
		t.Fatalf("checked native save = %+v, %v", stored, err)
	}
}

func TestSaveSessionNativeUnsupportedKeepsOptionDialog(t *testing.T) {
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
	state.saveSessionPicker = app.SaveFileWithCheckbox
	if err := driver.QueueSaveFileWithCheckbox("", false, app.ErrUnsupported); err != nil {
		t.Fatal(err)
	}
	state.command("Save Session...")
	if !state.saveSessionOptionsOpen {
		t.Fatal("unsupported native picker did not open option dialog")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "fallback.xml")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveSessionConfirmID)
	if _, err := session.LoadNPP(path); err != nil {
		t.Fatalf("fallback did not save: %v", err)
	}
	if err := driver.QueueSaveFile(filepath.Join(root, "ordinary.xml"), nil); err != nil {
		t.Fatal(err)
	}
	state.command("Save Session...")
	if !state.saveSessionOptionsOpen {
		t.Fatal("ordinary driver response did not leave the option dialog available")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveSessionCancelID)
	if remaining, err := app.SaveFile(app.SaveFileOptions{Title: "remaining response"}); err != nil || remaining != filepath.Join(root, "ordinary.xml") {
		t.Fatalf("ordinary response was consumed by checkbox request: %q, %v", remaining, err)
	}
}

func TestSaveSessionNativePickerCancelAndWorkspaceFailureAreSafe(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "first.txt")
	folder := filepath.Join(root, "workspace")
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
	state.saveSessionPicker = app.SaveFileWithCheckbox
	if err := driver.QueueSaveFileWithCheckbox("", false, app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	state.command("Save Session...")
	if state.saveSessionOptionsOpen || state.space.Current().Dirty() {
		t.Fatal("picker cancellation changed the workspace")
	}
	if err := os.Remove(folder); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "session.xml")
	if err := driver.QueueSaveFileWithCheckbox(path, true, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Save Session...")
	if !strings.Contains(state.notice, "folder unavailable") {
		t.Fatalf("invalid workspace notice = %q", state.notice)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid workspace wrote a session: %v", err)
	}
}
