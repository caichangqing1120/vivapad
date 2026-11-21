package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
)

func TestExpandedWorkspaceAutoRefreshesCreateRenameAndRemove(t *testing.T) {
	root := t.TempDir()
	state := newModel()
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	canonical := state.folderRoots[0]
	state.folderExpanded[canonical] = true
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if state.folderWatcher == nil || state.folderWatching[canonical] == nil {
		t.Fatal("expanded root was not watched")
	}
	contains := func(path string) bool {
		for _, row := range state.folderRows() {
			if row.path == path {
				return true
			}
		}
		return false
	}
	wait := func(want, absent string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			if (want == "" || contains(want)) && (absent == "" || !contains(absent)) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("tree did not refresh: want=%q absent=%q rows=%+v", want, absent, state.folderRows())
	}
	first := filepath.Join(canonical, "first.txt")
	if err := os.WriteFile(first, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	wait(first, "")
	renamed := filepath.Join(canonical, "renamed.txt")
	if err := os.Rename(first, renamed); err != nil {
		t.Fatal(err)
	}
	wait(renamed, first)
	if err := os.Remove(renamed); err != nil {
		t.Fatal(err)
	}
	wait("", renamed)
	state.activateFolderRow(folderRow{path: canonical, directory: true})
	if state.folderWatching[canonical] != nil {
		t.Fatal("collapsed root still holds a watch")
	}
	state.command("Folder as Workspace")
	if state.folderWatcher != nil {
		t.Fatal("hidden dock kept native watcher open")
	}
	state.command("Folder as Workspace")
	state.folderExpanded[canonical] = true
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.folderWatcher == nil || state.folderWatching[canonical] == nil {
		t.Fatal("reopened dock did not resume watching")
	}
}

func TestCollapsedBranchRereadsDiskAndWindowCloseReleasesWatches(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	canonical := state.folderRoots[0]
	canonicalChild := filepath.Join(canonical, "child")
	state.folderExpanded[canonical], state.folderExpanded[canonicalChild] = true, true
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if state.folderWatching[canonical] == nil || state.folderWatching[canonicalChild] == nil {
		t.Fatalf("expanded ancestors were not watched: %+v", state.folderWatching)
	}
	state.activateFolderRow(folderRow{path: canonicalChild, directory: true})
	if state.folderWatching[canonicalChild] != nil || state.folderWatching[canonical] == nil {
		t.Fatalf("collapse did not release only its branch: %+v", state.folderWatching)
	}
	newFile := filepath.Join(canonicalChild, "while-collapsed.txt")
	if err := os.WriteFile(newFile, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	state.activateFolderRow(folderRow{path: canonicalChild, directory: true})
	found := false
	for _, row := range state.folderRows() {
		found = found || row.path == newFile
	}
	if !found || state.folderWatching[canonicalChild] == nil {
		t.Fatalf("reopened branch reused stale cache: rows=%+v watches=%+v", state.folderRows(), state.folderWatching)
	}
	driver.Close()
	if state.folderWatcher != nil || len(state.folderWatching) != 0 {
		t.Fatal("window close leaked folder watches")
	}
}

func TestFolderRefreshWorksAfterWatcherFailure(t *testing.T) {
	root := t.TempDir()
	state := newModel()
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	canonical := state.folderRoots[0]
	state.folderExpanded[canonical] = true
	state.folderWatchUnavailable = true
	state.folderRows()
	if state.folderWatcher != nil {
		t.Fatal("simulated watcher failure unexpectedly created a watcher")
	}
	file := filepath.Join(canonical, "after-error.txt")
	if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state.refreshFolderTree()
	found := false
	for _, row := range state.folderRows() {
		found = found || row.path == file
	}
	if !found || state.folderWatcher == nil || state.folderWatching[canonical] == nil {
		t.Fatalf("manual refresh did not recover: found=%v watcher=%p watched=%+v", found, state.folderWatcher, state.folderWatching)
	}
	state.stopFolderWatching()
}

func TestWorkspaceReopenRefreshesChangesWhileUnwatched(t *testing.T) {
	root := t.TempDir()
	oldPath, newPath := filepath.Join(root, "old.txt"), filepath.Join(root, "new.txt")
	if err := os.WriteFile(oldPath, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	canonical := state.folderRoots[0]
	state.folderExpanded[canonical] = true
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	rows := func() (old, fresh bool) {
		for _, hit := range driver.Snapshot().Hits {
			old = old || hit.Label == "Open old.txt"
			fresh = fresh || hit.Label == "Open new.txt"
		}
		return old, fresh
	}
	if old, fresh := rows(); !old || fresh || state.folderWatching[canonical] == nil {
		t.Fatal("initial expanded tree was not shown and watched")
	}
	clickID(t, driver, folderCloseID)
	if state.folderVisible || state.folderWatcher != nil {
		t.Fatal("closing the dock did not stop watching")
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	state.command("Folder as Workspace")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if old, fresh := rows(); old || !fresh || state.folderWatching[canonical] == nil {
		t.Fatalf("reopened dock used stale rows: old=%v new=%v watched=%v", old, fresh, state.folderWatching[canonical])
	}
}

func TestExpandedWorkspaceWatchesReplacementAtTheSamePath(t *testing.T) {
	base := t.TempDir()
	root, replacement := filepath.Join(base, "root"), filepath.Join(base, "replacement")
	branch := filepath.Join(root, "branch")
	for _, path := range []string{branch, replacement} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(replacement, "replacement.txt"), []byte("new folder"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	canonical := state.folderRoots[0]
	canonicalBranch := filepath.Join(canonical, "branch")
	state.folderExpanded[canonical], state.folderExpanded[canonicalBranch] = true, true
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := os.Rename(branch, filepath.Join(base, "parked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, branch); err != nil {
		t.Fatal(err)
	}
	waitForRow := func(name string) {
		t.Helper()
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			for _, hit := range driver.Snapshot().Hits {
				if hit.Label == "Open "+name {
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("expanded replacement did not show %q", name)
	}
	waitForRow("replacement.txt")
	if err := os.WriteFile(filepath.Join(branch, "later.txt"), []byte("another change"), 0600); err != nil {
		t.Fatal(err)
	}
	waitForRow("later.txt")
}
