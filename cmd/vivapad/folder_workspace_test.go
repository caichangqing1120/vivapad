package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/session"
)

func TestFolderWorkspacePickerTreeOpenRefreshAndHide(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "src")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(child, "main.go")
	if err := os.WriteFile(file, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	draft := state.space.Current()
	draft.Editor.Insert("keep draft")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenDirectory("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(16))
	if state.folderVisible || len(state.folderRoots) != 0 || len(state.space.Documents) != 1 || draft.Editor.String() != "keep draft" {
		t.Fatal("canceled picker changed workspace")
	}
	if err := driver.QueueOpenDirectory(root, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(16))
	if !state.folderVisible || len(state.folderRoots) != 1 {
		t.Fatalf("folder picker did not open dock: %+v", state.folderRoots)
	}
	clickID(t, driver, folderRowScope.At(0))
	clickID(t, driver, folderRowScope.At(1))
	clickID(t, driver, folderRowScope.At(2))
	if state.space.Current().Editor.String() != "package main" || state.space.Current().Path == "" || state.documentIndex(draft) < 0 || !draft.Dirty() {
		t.Fatalf("file row did not open in editor: %+v", state.space.Current())
	}
	newFile := filepath.Join(child, "next.go")
	if err := os.WriteFile(newFile, []byte("next"), 0600); err != nil {
		t.Fatal(err)
	}
	hasFile := func() bool {
		for _, row := range state.folderRows() {
			if row.path == filepath.Join(state.folderRoots[0], "src", "next.go") {
				return true
			}
		}
		return false
	}
	if hasFile() {
		t.Fatal("tree changed before explicit refresh")
	}
	clickID(t, driver, folderRefreshID)
	if !hasFile() {
		t.Fatal("refresh did not show new file")
	}
	secondRoot := t.TempDir()
	if err := driver.QueueOpenDirectory(secondRoot, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, folderAddID)
	if len(state.folderRoots) != 2 {
		t.Fatalf("Add folder did not append a root: %+v", state.folderRoots)
	}
	clickID(t, driver, folderCloseID)
	if state.folderVisible || len(state.folderRoots) != 2 {
		t.Fatal("closing dock discarded its roots")
	}
	state.command("Folder as Workspace")
	if !state.folderVisible {
		t.Fatal("View command did not reopen dock")
	}
}

func TestFolderWorkspaceFocusesEditorOnlyAfterSuccessfulFileOpen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		split bool
		focus gfx.ID
	}{{"single pane", false, editorID}, {"split pane", true, rightEditorID}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "editable.txt")
			const raw = "saved\r\n"
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "binary.txt"), []byte{'a', 0, 'b'}, 0600); err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			draft := state.space.Current()
			draft.Editor.Insert("keep draft")
			if tc.split {
				state.space.NewDocument()
				state.toggleSplit()
			}
			if err := state.addFolderRoot(root); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(state.stopFolderWatching)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, folderRowScope.At(0))
			if got := driver.Snapshot().Focused; got.Scope() != folderRowScope {
				t.Fatalf("expanding a folder moved focus out of the tree: %v", got)
			}
			previous := state.space.Current()
			clickLabeledControl(t, driver, "Open binary.txt")
			if got := driver.Snapshot().Focused; got.Scope() != folderRowScope || state.space.Current() != previous || !strings.Contains(state.notice, "Cannot open binary.txt") {
				t.Fatalf("refused file open changed focus or document: focus=%v notice=%q", got, state.notice)
			}
			clickLabeledControl(t, driver, "Open editable.txt")
			opened := state.space.Current()
			if got := driver.Snapshot().Focused; got != tc.focus || opened.Name != "editable.txt" {
				t.Fatalf("file open focused %v on %s; want %v on editable.txt", got, opened.Name, tc.focus)
			}
			if err := driver.Type("x"); err != nil {
				t.Fatal(err)
			}
			if opened.Editor.String() != "xsaved\n" || draft.Editor.String() != "keep draft" {
				t.Fatalf("first key routed incorrectly: opened=%q draft=%q", opened.Editor.String(), draft.Editor.String())
			}
			if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || opened.Editor.String() != "saved\n" || opened.Dirty() {
				t.Fatalf("undo of first key: text=%q dirty=%v handled=%v err=%v", opened.Editor.String(), opened.Dirty(), handled, err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != raw {
				t.Fatalf("file bytes changed: %q, %v", data, err)
			}
		})
	}
}

func TestFolderWorkspaceRejectsNestedDuplicatesAndInvalidCurrentFile(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "src")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(child, "main.go")
	if err := os.WriteFile(file, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	state.command("Folder of Current Document as Workspace")
	if len(state.folderRoots) != 0 || !strings.Contains(state.notice, "Save") {
		t.Fatalf("untitled folder command = %+v, %q", state.folderRoots, state.notice)
	}
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	canonicalChild, _ := filepath.EvalSymlinks(child)
	if err := state.addFolderRoot(child); err != nil || len(state.folderRoots) != 1 || state.folderSelected != canonicalChild {
		t.Fatalf("nested folder was duplicated: %+v, selected=%q, err=%v", state.folderRoots, state.folderSelected, err)
	}
	if err := state.addFolderRoot(root); err != nil || len(state.folderRoots) != 1 {
		t.Fatalf("duplicate root was appended: %+v, %v", state.folderRoots, err)
	}
	if err := state.addFolderRoot(filepath.Dir(root)); err == nil || len(state.folderRoots) != 1 {
		t.Fatalf("overlapping parent was accepted: %+v, %v", state.folderRoots, err)
	}
	saved := newModelWithFiles([]string{file})
	saved.command("Folder of Current Document as Workspace")
	if len(saved.folderRoots) != 1 || saved.folderRoots[0] != canonicalChild || !saved.folderVisible {
		t.Fatalf("saved document parent was not opened as workspace: %+v, %q", saved.folderRoots, saved.notice)
	}
}

func TestFolderWorkspaceRestoresRootsAndSkipsMissingOnRestart(t *testing.T) {
	base := t.TempDir()
	first, missing := filepath.Join(base, "first"), filepath.Join(base, "missing")
	for _, path := range []string{first, missing} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	privatePath := filepath.Join(base, "settings", "session.json")
	state := newStartupModel(nil, privatePath)
	if err := state.addFolderRoot(first); err != nil {
		t.Fatal(err)
	}
	if err := state.addFolderRoot(missing); err != nil {
		t.Fatal(err)
	}
	state.folderVisible = false
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, privatePath)
	canonicalFirst, _ := filepath.EvalSymlinks(first)
	if !allowed || restored.folderVisible || len(restored.folderRoots) != 1 || restored.folderRoots[0] != canonicalFirst {
		t.Fatalf("folder restore = %+v, visible=%v, notice=%q", restored.folderRoots, restored.folderVisible, restored.notice)
	}
}

func TestFolderWorkspaceMenuAndSplitTreeScrollFitCompactWindow(t *testing.T) {
	root := t.TempDir()
	for index := range 30 {
		path := filepath.Join(root, fmt.Sprintf("file-%02d.txt", index))
		if err := os.WriteFile(path, []byte("text"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	workspaceRow := revealFileMenuItem(t, driver, 18)
	if workspaceRow.Y+workspaceRow.Height > 400-statusHeight {
		t.Fatalf("workspace menu overlaps status: %+v", workspaceRow)
	}
	if workspaceRow.Width < 270 {
		t.Fatal("File menu does not fit workspace command label")
	}
	clickID(t, driver, menuItemScope.At(18))
	clickID(t, driver, containingFolderScope.At(3))
	if !strings.Contains(state.notice, "Save") {
		t.Fatalf("last File menu command was not reachable: %q", state.notice)
	}
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	state.folderExpanded[state.folderRoots[0]] = true
	state.toggleSplit()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	panelWidth := state.folderPanelWidth(640)
	var splitter, leftEditor gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == splitterID {
			splitter = hit.Bounds
		}
		if hit.ID == editorID {
			leftEditor = hit.Bounds
		}
	}
	if leftEditor.X != panelWidth || splitter.X <= leftEditor.X || splitter.X+splitter.Width >= 640 {
		t.Fatalf("split editor not aligned beside dock: panel=%g editor=%+v splitter=%+v", panelWidth, leftEditor, splitter)
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: 40, Y: 210, DeltaY: 120}); err != nil {
		t.Fatal(err)
	}
	if state.folderOffset == 0 {
		t.Fatal("wheel did not scroll the folder tree")
	}
}

func TestFolderWorkspaceKeepsVisibleDockWhenRootDisappears(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "removed")
	privatePath := filepath.Join(root, "settings", "session.json")
	if err := session.Save(privatePath, session.State{FolderRoots: []string{missing}, FolderVisible: true}); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, privatePath)
	if !state.folderVisible || len(state.folderRoots) != 0 || !strings.Contains(state.notice, "Folder unavailable") {
		t.Fatalf("missing root visibility = %v, roots=%+v, notice=%q", state.folderVisible, state.folderRoots, state.notice)
	}
}
