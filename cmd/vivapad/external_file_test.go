package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/external"
)

func TestOpenInDefaultViewerPreservesDirtyBufferAndRejectsUnsafeFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("on disk"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	doc := state.space.Current()
	doc.Editor.Insert("draft")
	state.command("Open in Default Viewer")
	opened, ok := driver.LastOpenedPath()
	if !ok || opened != doc.Path || !doc.Dirty() || doc.Editor.String() != "drafton disk" {
		t.Fatalf("default viewer changed editor or path: opened=%q ok=%v dirty=%v text=%q", opened, ok, doc.Dirty(), doc.Editor.String())
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "on disk" {
		t.Fatalf("default viewer wrote file: %q, %v", data, err)
	}
	state.showInActivePane(state.space.NewDocument())
	state.command("Open in Default Viewer")
	if !strings.Contains(state.notice, "Save") {
		t.Fatalf("untitled viewer notice = %q", state.notice)
	}
	unsafeName := "run.command"
	if runtime.GOOS == "windows" {
		unsafeName = "run.cmd"
	} else if runtime.GOOS == "linux" {
		unsafeName = "run.desktop"
	}
	unsafePath := filepath.Join(root, unsafeName)
	if err := os.WriteFile(unsafePath, []byte("do not launch"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(unsafePath); err != nil {
		t.Fatal(err)
	}
	state.showInActivePane(state.space.Current())
	state.command("Open in Default Viewer")
	if opened, _ := driver.LastOpenedPath(); opened != doc.Path || !strings.Contains(state.notice, "would be run") {
		t.Fatalf("unsafe path was handed to default viewer: opened=%q notice=%q", opened, state.notice)
	}
}

func TestContainingFolderConsoleMenusPreserveEditor(t *testing.T) {
	for _, tc := range []struct {
		label string
		index int
		kind  external.ConsoleKind
		fail  bool
	}{
		{"cmd", 1, external.CommandPrompt, false},
		{"PowerShell", 2, external.WindowsPowerShell, true},
	} {
		t.Run(tc.label, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "source & demo.txt")
			if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{file})
			doc := state.space.Current()
			doc.Editor.Insert("draft")
			before, selection := doc.Editor.String(), doc.Editor.Selection()
			language := state.languageFor(doc)
			calls := 0
			state.openConsole = func(kind external.ConsoleKind, path string) error {
				calls++
				if kind != tc.kind || path != doc.Path {
					t.Fatalf("console launch target = %d %q, want %d %q", kind, path, tc.kind, doc.Path)
				}
				if tc.fail {
					return errors.New("shell unavailable")
				}
				return nil
			}
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickConsole := func() {
				clickID(t, driver, menuScope.At(0))
				clickID(t, driver, menuItemScope.At(18))
				clickID(t, driver, containingFolderScope.At(tc.index))
			}
			clickConsole()
			requireEditorCommandFocus(t, driver, editorID)
			if calls != 1 || state.menuOpen >= 0 || state.space.Current() != doc ||
				!doc.Dirty() || doc.Editor.String() != before || doc.Editor.Selection() != selection || state.languageFor(doc) != language {
				t.Fatalf("launch altered document or menu state: calls=%d notice=%q", calls, state.notice)
			}
			if tc.fail && !strings.Contains(state.notice, "shell unavailable") {
				t.Fatalf("failed launch notice = %q", state.notice)
			}
			if data, err := os.ReadFile(file); err != nil || string(data) != "saved" {
				t.Fatalf("launch changed source bytes: %q, %v", data, err)
			}
			state.showInActivePane(state.space.NewDocument())
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickConsole()
			if calls != 1 || !strings.Contains(state.notice, "Save") {
				t.Fatalf("untitled launch: calls=%d notice=%q", calls, state.notice)
			}
		})
	}
}

func TestContainingFolderSubmenuFitsCompactViewportAndUsesSavedPath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "selected.txt")
	if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	var revealed string
	state.revealPath = func(path string) error { revealed = path; return nil }
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(18))
	for index := range containingFolderCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == containingFolderScope.At(index) {
				found = true
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight || hit.Disabled != (runtime.GOOS != "windows" && (index == 1 || index == 2)) {
					t.Fatalf("containing-folder row %d invalid: %+v", index, hit)
				}
			}
		}
		if !found {
			t.Fatalf("containing-folder row %d missing", index)
		}
	}
	clickID(t, driver, containingFolderScope.At(0))
	if revealed != state.space.Current().Path {
		t.Fatalf("Explorer reveal = %q, want %q", revealed, state.space.Current().Path)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(18))
	clickID(t, driver, containingFolderScope.At(3))
	if len(state.folderRoots) != 1 || !state.folderVisible || state.space.Current().Path != revealed {
		t.Fatalf("folder submenu changed document or missed workspace: roots=%+v current=%q", state.folderRoots, state.space.Current().Path)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(18))
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.containingMenuOpen || state.menuOpen != 0 {
		t.Fatal("Escape did not return to the parent File menu")
	}
	state.revealPath = func(string) error { return errors.New("file manager denied") }
	state.command("Explorer")
	if !strings.Contains(state.notice, "file manager denied") || state.space.Current().Path != revealed {
		t.Fatalf("reveal error changed the document: %q", state.notice)
	}
}
