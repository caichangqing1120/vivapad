package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vivapad/internal/session"
)

func workspaceFixtureLink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		if runtime.GOOS == "windows" || errors.Is(err, fs.ErrPermission) {
			t.Skipf("symbolic links unavailable: %v", err)
		}
		t.Fatal(err)
	}
}

func TestWorkspaceDoesNotFollowReplacedRoot(t *testing.T) {
	base := t.TempDir()
	root, other := filepath.Join(base, "root"), filepath.Join(base, "other")
	for _, path := range []string{root, other} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(other, "unrelated.txt"), []byte("unrelated"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	canonical := state.folderRoots[0]
	privatePath := filepath.Join(base, "settings", "session.json")
	if err := session.Save(privatePath, session.State{FolderRoots: []string{canonical}, FolderVisible: true}); err != nil {
		t.Fatal(err)
	}
	state.folderExpanded[canonical] = true
	state.folderRows()
	state.command("Folder as Workspace")
	if err := os.Rename(root, filepath.Join(base, "parked")); err != nil {
		t.Fatal(err)
	}
	workspaceFixtureLink(t, other, root)
	t.Run("reopen", func(t *testing.T) {
		state.command("Folder as Workspace")
		defer state.stopFolderWatching()
		if rows := state.folderRows(); len(rows) != 1 || state.folderWatching[canonical] || !strings.Contains(state.notice, "Folder unavailable") {
			t.Fatalf("reopened redirected root: rows=%+v watched=%v notice=%q", rows, state.folderWatching[canonical], state.notice)
		}
	})
	t.Run("restart", func(t *testing.T) {
		restored := newStartupModel(nil, privatePath)
		if !restored.folderVisible || len(restored.folderRoots) != 0 || !strings.Contains(restored.notice, "Folder unavailable") {
			t.Fatalf("restored redirected root: visible=%v roots=%v notice=%q", restored.folderVisible, restored.folderRoots, restored.notice)
		}
	})
}

func TestWorkspaceRefusesFileRowRedirectedAfterListing(t *testing.T) {
	base, state := t.TempDir(), newModel()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	file, outside := filepath.Join(root, "note.txt"), filepath.Join(base, "other.txt")
	for _, path := range []string{file, outside} {
		if err := os.WriteFile(path, []byte(path), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	state.folderExpanded[state.folderRoots[0]] = true
	t.Cleanup(state.stopFolderWatching)
	rows := state.folderRows()
	if len(rows) != 2 || rows[1].directory {
		t.Fatalf("fixture did not list the original file: %+v", rows)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	workspaceFixtureLink(t, outside, file)
	before, count := state.space.Current(), len(state.space.Documents)
	state.activateFolderRow(rows[1])
	if state.space.Current() != before || len(state.space.Documents) != count || !strings.Contains(state.notice, "Cannot open") {
		t.Fatalf("stale row opened redirected file: tabs=%d notice=%q", len(state.space.Documents), state.notice)
	}
}
