package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"vivapad/internal/foldertree"
)

func watchedFolderRows(t testing.TB, count int) (*model, string) {
	t.Helper()
	state := newModel()
	if err := state.addFolderRoot(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(state.stopFolderWatching)
	root := state.folderRoots[0]
	state.folderExpanded[root] = true
	state.ensureFolderWatch(root)
	if state.folderWatching[root] == nil {
		t.Fatalf("fixture could not watch root: %s", state.notice)
	}
	entries := make([]foldertree.Entry, count)
	for i := range entries {
		name := fmt.Sprintf("file%05d.txt", i)
		entries[i] = foldertree.Entry{Name: name, Path: filepath.Join(root, name)}
	}
	state.folderCache[root] = entries
	if got := len(state.folderRows()); got != count+1 {
		t.Fatalf("fixture returned %d rows, want %d", got, count+1)
	}
	return state, root
}

func TestFolderWorkspaceSteadyRowsDoNotAllocate(t *testing.T) {
	state, root := watchedFolderRows(t, 1024)
	if allocs := testing.AllocsPerRun(10, func() { state.folderRows() }); allocs != 0 {
		t.Errorf("unchanged watched tree allocated %.0f times per read", allocs)
	}
	state.activateFolderRow(folderRow{path: root, directory: true})
	if got := len(state.folderRows()); got != 1 {
		t.Fatalf("collapsed root returned %d rows, want 1", got)
	}
	state.activateFolderRow(folderRow{path: root, directory: true})
	if got := len(state.folderRows()); got != 1 || state.folderWatching[root] == nil {
		t.Fatalf("reopening the empty on-disk root reused old entries or lost its watch: rows=%d", got)
	}
}

func BenchmarkFolderWorkspaceSteadyRows(b *testing.B) {
	state, _ := watchedFolderRows(b, 16_384)
	b.ReportAllocs()
	for b.Loop() {
		state.folderRows()
	}
}
