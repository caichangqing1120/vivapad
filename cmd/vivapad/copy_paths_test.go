package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestCopyCurrentPathNameAndDirectoryFromSavedUnicodeFile(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "猫 one.txt")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	before := doc.Editor.Selection()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	for _, test := range []struct {
		index int
		want  string
	}{{0, path}, {1, "猫 one.txt"}, {2, root}} {
		clickID(t, driver, menuScope.At(1))
		clickID(t, driver, menuItemScope.At(13))
		clickID(t, driver, copyPathsScope.At(test.index))
		if got := driver.Clipboard(); got != test.want || doc.Editor.Selection() != before || doc.Dirty() || doc.Editor.CanUndo() {
			t.Fatalf("copy item %d = %q, want %q; selection=%+v dirty=%v", test.index, got, test.want, doc.Editor.Selection(), doc.Dirty())
		}
	}
}

func TestCopyUntitledPseudoPathAndEmptyDirectory(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	state.command("Copy Current Full File path")
	if driver.Clipboard() != doc.Name || doc.Path != "" {
		t.Fatalf("untitled pseudo-path = %q, doc=%+v", driver.Clipboard(), doc)
	}
	state.command("Copy Current Filename")
	if driver.Clipboard() != doc.Name {
		t.Fatalf("untitled filename = %q", driver.Clipboard())
	}
	driver.SetClipboard("prior")
	state.command("Copy Current Dir. Path")
	if driver.Clipboard() != "" || doc.Dirty() {
		t.Fatalf("untitled directory clipboard = %q, dirty=%v", driver.Clipboard(), doc.Dirty())
	}
}

func TestCopyAllNamesAndPathsOrderViewsAndDeduplicateClone(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	firstPath, secondPath := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{firstPath, secondPath})
	first, second := state.space.Documents[0], state.space.Documents[1]
	untitled := state.space.NewDocument()
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{first, first}
	state.paneOwner[first], state.paneOwner[untitled] = 0, 0
	state.paneOwner[second] = 1
	state.activePane = 0
	state.space.Select(0)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Copy All Filenames")
	wantNames := "first.txt\r\n" + untitled.Name + "\r\nsecond.txt\r\n"
	if got := driver.Clipboard(); got != wantNames || state.space.Current() != first {
		t.Fatalf("all names = %q, want %q", got, wantNames)
	}
	state.command("Copy All File Paths")
	wantPaths := firstPath + "\r\n" + untitled.Name + "\r\n" + secondPath + "\r\n"
	if got := driver.Clipboard(); got != wantPaths || len(state.space.Documents) != 3 {
		t.Fatalf("all paths = %q, want %q", got, wantPaths)
	}
}

func TestCopyPathSubmenuFitsCompactWindowAndEscapes(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(13))
	visible := 0
	for _, hit := range driver.Snapshot().Hits {
		for index := range copyPathsCommands {
			if hit.ID == copyPathsScope.At(index) {
				visible++
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("copy path row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
	}
	if visible != len(copyPathsCommands) {
		t.Fatalf("visible copy-path rows = %d, want %d", visible, len(copyPathsCommands))
	}
	clickID(t, driver, copyPathsScope.At(3))
	if state.menuOpen != -1 || state.copyPathsMenuOpen || driver.Clipboard() != "new 1\r\n" {
		t.Fatalf("all-names action = menu %d open=%v clip=%q", state.menuOpen, state.copyPathsMenuOpen, driver.Clipboard())
	}
}
