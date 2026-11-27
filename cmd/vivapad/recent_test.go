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

func TestRecentMenuUsesSavedDecoderAndReturnsFocusToActivePane(t *testing.T) {
	for _, tc := range []struct {
		name            string
		pending, stored document.Encoding
		wantOpen        bool
	}{
		{"saved decoder", document.GB18030, document.GB18030, true},
		{"pending conversion", document.UTF8, document.GB18030, true},
		{"only pending conversion matches", document.Windows1252, document.Windows1252, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "saved legacy.txt")
			disk := []byte{0xd6, 0xd0, '\r', '\n'}
			if err := os.WriteFile(path, disk, 0600); err != nil {
				t.Fatal(err)
			}
			state := newModel()
			if err := state.space.OpenWithEncoding(path, document.GB18030); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			doc.SetEncoding(tc.pending)
			doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
			left, right := state.space.NewDocument(), state.space.NewDocument()
			left.Editor.Insert("left draft")
			right.Editor.Insert("right draft")
			state.paneOwner[left], state.paneOwner[doc], state.paneOwner[right] = 0, 1, 1
			state.paneDocs = [2]*document.Document{left, right}
			state.splitEnabled, state.activePane = true, 1
			state.recentEntries = []recent.Entry{{Path: doc.Path, Encoding: tc.stored}}
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, rightEditorID)
			clickID(t, driver, menuScope.At(0))
			clickID(t, driver, menuItemScope.At(10))
			clickID(t, driver, recentScope.At(0))
			want := right
			if tc.wantOpen {
				want = doc
			}
			if state.space.Current() != want || state.paneDocument(1) != want || state.paneDocument(0) != left {
				t.Fatalf("unexpected current tab: %q, notice=%q", state.space.Current().Name, state.notice)
			}
			if doc.Encoding() != tc.pending || doc.SavedEncoding() != document.GB18030 || doc.Editor.String() != "\u4e2d\n" || doc.Editor.CanUndo() {
				t.Fatal("history changed the existing buffer, its encodings, or Undo state")
			}
			if !tc.wantOpen && !strings.Contains(state.notice, "Cannot open recent") {
				t.Fatalf("missing decoder mismatch notice: %q", state.notice)
			}
			if got := driver.Snapshot().Focused; got != rightEditorID {
				t.Errorf("focus after recent open = %v, want the right editor", got)
			}
			if tc.wantOpen {
				if err := driver.Type("x"); err != nil {
					t.Fatal(err)
				}
				if doc.Editor.String() != "\u4e2d\nx" || right.Editor.String() != "right draft" || left.Editor.String() != "left draft" {
					t.Fatal("first character after selection did not reach the reopened document")
				}
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != string(disk) {
				t.Fatalf("recent open changed the saved file: %x, %v", data, err)
			}
		})
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

func TestClearRecentKeepsMemoryConsistentAfterPartialFailure(t *testing.T) {
	root := t.TempDir()
	state := newStartupModel(nil, filepath.Join(root, "settings", "session.json"))
	original := state.space.Current()
	state.recentEntries = []recent.Entry{{Path: filepath.Join(root, "recent.txt")}}
	state.closedHistory = []recent.Entry{{Path: filepath.Join(root, "closed.txt")}}
	if err := recent.Save(state.closedPath, state.closedHistory); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(state.recentPath, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(state.recentPath, "keep")
	if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}

	state.clearRecent()
	if _, err := os.Stat(state.closedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("closed history remained on disk: %v", err)
	}
	if len(state.closedHistory) != 0 {
		t.Fatalf("deleted closed history remained available in memory: %+v", state.closedHistory)
	}
	if len(state.recentEntries) != 1 || state.recentEntries[0].Path != filepath.Join(root, "recent.txt") ||
		state.space.Current() != original || !strings.Contains(state.notice, "Recent files not cleared") {
		t.Fatalf("partial clear lost recent entries or its error: entries=%+v notice=%q", state.recentEntries, state.notice)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(state.recentPath); err != nil {
		t.Fatal(err)
	}
	if err := recent.Save(state.recentPath, state.recentEntries); err != nil {
		t.Fatal(err)
	}
	state.clearRecent()
	if len(state.recentEntries) != 0 || len(state.closedHistory) != 0 || state.space.Current() != original {
		t.Fatalf("retry did not finish clearing: recent=%+v closed=%+v", state.recentEntries, state.closedHistory)
	}
	if _, err := os.Stat(state.recentPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retry left recent history on disk: %v", err)
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

func TestRecentSubmenuKeyboardNavigationRestoresFile(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "closed.txt")
	if err := os.WriteFile(path, []byte("closed text"), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.recentEntries = recent.Add(nil, recent.Entry{Path: filepath.Join(root, "recent.txt")})
	state.closedHistory = recent.Add(nil, recent.Entry{Path: path})
	original := state.space.Current()
	parent := menuItemScope.At(10)
	press := func(key gfx.Key, want gfx.ID) {
		t.Helper()
		if err := driver.Key(key, 0); err != nil {
			t.Fatal(err)
		}
		if got := driver.Snapshot().Focused; got != want {
			t.Fatalf("after key %v focus = %v, want %v", key, got, want)
		}
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, parent)
	press(gfx.KeyDown, recentScope.At(0))
	press(gfx.KeyUp, recentClearID)
	press(gfx.KeyEscape, parent)
	if state.recentOpen || state.space.Current() != original || len(state.closedHistory) != 1 {
		t.Fatal("Escape did not return to the parent without changing state")
	}
	press(gfx.KeyRight, recentScope.At(0))
	press(gfx.KeyEnd, recentClearID)
	press(gfx.KeyUp, recentRestoreID)
	press(gfx.KeyEnter, editorID)
	if state.recentOpen || state.space.Current().Path != path || len(state.closedHistory) != 0 {
		t.Fatal("Enter did not restore the selected closed file")
	}
	if err := driver.Type("x"); err != nil || state.space.Current().Editor.String() != "xclosed text" {
		t.Fatalf("first key did not reach the restored editor: %v", err)
	}
}

func TestRecentMenuRejectsHeldClickAfterFileDrop(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, second, third := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt"), filepath.Join(root, "third.txt")
	for _, path := range []string{first, second, third} {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, driver := startDriver(t)
	drop := func(path string) {
		t.Helper()
		ok, err := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, X: 500, Y: 250, Paths: []string{path}})
		if err != nil || !ok || state.space.Current().Path != path {
			t.Fatalf("drop %q: accepted=%v err=%v active=%q", path, ok, err, state.space.Current().Path)
		}
	}
	openMenu := func() {
		t.Helper()
		clickID(t, driver, menuScope.At(0))
		clickID(t, driver, menuItemScope.At(10))
	}
	drop(first)
	drop(second)
	openMenu()
	missing := filepath.Join(root, "missing.txt")
	if _, err := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, X: 500, Y: 250, Paths: []string{missing}}); err != nil {
		t.Fatal(err)
	}
	if !state.recentOpen || state.space.Current().Path != second || !strings.Contains(state.notice, "missing.txt") {
		t.Fatal("a failed drop did not keep the current document, menu and error")
	}
	var bounds gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID.Scope() == recentScope && hit.Label == first {
			bounds = hit.Bounds
		}
	}
	if bounds.Width <= 0 {
		t.Fatal("first file was not shown in Recent Files")
	}
	x, y := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
	if err := driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	drop(third)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.recentEntries[1].Path != second {
		t.Fatal("the file drop did not replace the pressed row")
	}
	if err := driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if state.space.Current().Path != third {
		t.Fatalf("the stale menu release activated %q instead of keeping the dropped file active", state.space.Current().Path)
	}
	if !state.recentOpen {
		openMenu()
	}
	clickLabeledControl(t, driver, second)
	if state.space.Current().Path != second {
		t.Fatal("a fresh menu click did not activate the displayed file")
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
	state.replaceFileResults(searchfiles.Result{Matches: []searchfiles.Match{{Path: found, Source: "find", Start: 0, End: 4, Text: "find"}}})
	state.openFileResult(0)
	if len(state.recentEntries) != 2 || filepath.Base(state.recentEntries[0].Path) != "found.txt" || state.space.Current().Editor.String() != "find" {
		t.Fatalf("find result recent = %+v current=%+v", state.recentEntries, state.space.Current())
	}
}
