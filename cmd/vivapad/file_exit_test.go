package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"vivapad/internal/session"
)

func TestFileExitUsesNativeDirtyCloseGate(t *testing.T) {
	root := t.TempDir()
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel(nil, sessionPath)
	config := newConfig(state)
	driver, err := app.NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	closed := 0
	state.closeWindow = func() { config.OnCloseRequest(func() { closed++ }) }
	doc := state.space.Current()
	doc.Editor.Insert("unsaved")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	row := revealFileMenuItem(t, driver, 23)
	if row.Y+row.Height > 400-statusHeight {
		t.Fatalf("Exit overlaps status: %+v", row)
	}
	clickID(t, driver, menuItemScope.At(23))
	if closed != 0 || state.pendingClose < 0 || !doc.Dirty() {
		t.Fatal("Exit bypassed the dirty close gate")
	}
	clickID(t, driver, cancelCloseID)
	if closed != 0 || doc.Editor.String() != "unsaved" {
		t.Fatal("Cancel did not keep the window and buffer")
	}
	if _, err := os.Stat(sessionPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled Exit persisted session: %v", err)
	}
	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(23))
	clickID(t, driver, saveCloseID)
	if closed != 0 || state.pendingClose < 0 || doc.Editor.String() != "unsaved" {
		t.Fatal("canceled Save As closed or changed the document")
	}
	clickID(t, driver, discardID)
	if closed != 1 || state.pendingClose >= 0 {
		t.Fatal("explicit Discard did not allow native close")
	}
	if _, err := session.Load(sessionPath); err != nil {
		t.Fatalf("confirmed Exit did not persist session: %v", err)
	}
}

func TestCaptureFileExitWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_FILE_EXIT_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_FILE_EXIT_SHOTS to capture the File menu")
	}
	state := newModel()
	state.space.Current().Editor.Insert("unsaved draft")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), menuScope.At(0))
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "file-exit-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), menuScope.At(0))
	}
}
