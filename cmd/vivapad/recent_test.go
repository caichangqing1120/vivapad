package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/recent"
	"vivapad/internal/searchfiles"
	"vivapad/internal/session"
)

func TestRecentMenuRecordsSaveAndReopensLegacyFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "chinese.txt")
	saved := filepath.Join(root, "draft.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte{0xd6, 0xd0}, 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(7))
	if len(state.recentEntries) != 1 || state.recentEntries[0].Encoding != document.GB18030 {
		t.Fatalf("recent legacy open = %+v", state.recentEntries)
	}
	clickID(t, driver, newID)
	clickID(t, driver, editorID)
	if err := driver.Type("draft"); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueSaveFile(saved, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveID)
	if len(state.recentEntries) != 2 || filepath.Base(state.recentEntries[0].Path) != "draft.txt" {
		t.Fatalf("Save As did not update recent order: %+v", state.recentEntries)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, recentScope.At(1))
	if state.space.Current().Editor.String() != "中" || state.space.Current().Encoding() != document.GB18030 || state.recentOpen || filepath.Base(state.recentEntries[0].Path) != "chinese.txt" {
		t.Fatalf("recent legacy selection = %+v, recent=%+v", state.space.Current(), state.recentEntries)
	}
	loaded, err := recent.Load(filepath.Join(filepath.Dir(sessionPath), "recent.json"))
	if err != nil || len(loaded) != 2 || loaded[0].Encoding != document.GB18030 {
		t.Fatalf("persisted recent history = %+v, %v", loaded, err)
	}
}

func TestRecentMissingFileDoesNotMutateTabsAndCanClear(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "missing.txt")
	state := newStartupModel(nil, filepath.Join(root, "settings", "session.json"))
	state.recentEntries = recent.Add(nil, recent.Entry{Path: path})
	if err := recent.Save(state.recentPath, state.recentEntries); err != nil {
		t.Fatal(err)
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	original := state.space.Current()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, recentScope.At(0))
	if state.space.Current() != original || !strings.Contains(state.notice, "missing.txt") {
		t.Fatalf("missing recent changed tab: %+v notice=%q", state.space.Current(), state.notice)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, recentClearID)
	if len(state.recentEntries) != 0 || state.recentOpen {
		t.Fatalf("recent clear = %+v open=%v", state.recentEntries, state.recentOpen)
	}
	if _, err := os.Stat(state.recentPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clear left history file: %v", err)
	}
}

func TestRestoringSessionDoesNotReorderRecentButCLIFileDoes(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt"), filepath.Join(root, "third.txt")}
	for index, path := range paths {
		if err := os.WriteFile(path, []byte{byte('1' + index)}, 0600); err != nil {
			t.Fatal(err)
		}
		paths[index], _ = filepath.EvalSymlinks(path)
	}
	directory := filepath.Join(root, "settings")
	sessionPath := filepath.Join(directory, "session.json")
	if err := session.Save(sessionPath, session.State{Paths: paths[:2], Active: paths[0]}); err != nil {
		t.Fatal(err)
	}
	if err := recent.Save(filepath.Join(directory, "recent.json"), []recent.Entry{{Path: paths[1]}, {Path: paths[0]}}); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, sessionPath)
	if len(restored.recentEntries) != 2 || restored.recentEntries[0].Path != paths[1] {
		t.Fatalf("session restore reordered history: %+v", restored.recentEntries)
	}
	explicit := newStartupModel([]string{paths[2]}, sessionPath)
	if len(explicit.recentEntries) != 3 || explicit.recentEntries[0].Path != paths[2] {
		t.Fatalf("CLI open did not become recent: %+v", explicit.recentEntries)
	}
}

func TestRecentSubmenuClosesWithEscapeWithoutChangingDocument(t *testing.T) {
	state, driver := startDriver(t)
	state.recentEntries = recent.Add(nil, recent.Entry{Path: filepath.Join(t.TempDir(), "sample.txt")})
	original := state.space.Current()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(10))
	if !state.recentOpen {
		t.Fatal("recent submenu did not open")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.recentOpen || state.space.Current() != original {
		t.Fatalf("Escape changed document or left submenu open: %+v open=%v", state.space.Current(), state.recentOpen)
	}
}

func TestFileDropAndFindResultUpdateRecentHistory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dropped, found := filepath.Join(root, "dropped.txt"), filepath.Join(root, "found.txt")
	if err := os.WriteFile(dropped, []byte("drop"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(found, []byte("find"), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	accepted, err := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, X: 100, Y: 110, Paths: []string{dropped}})
	if err != nil || !accepted || len(state.recentEntries) != 1 || filepath.Base(state.recentEntries[0].Path) != "dropped.txt" {
		t.Fatalf("drop recent = %+v accepted=%v err=%v", state.recentEntries, accepted, err)
	}
	state.fileResults.Matches = []searchfiles.Match{{Path: found, Source: "find", Start: 0, End: 4, Text: "find"}}
	state.openFileResult(0)
	if len(state.recentEntries) != 2 || filepath.Base(state.recentEntries[0].Path) != "found.txt" || state.space.Current().Editor.String() != "find" {
		t.Fatalf("find result recent = %+v current=%+v", state.recentEntries, state.space.Current())
	}
}
