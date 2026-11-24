//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWorkspaceRefreshRejectsDirectoryReplacedByPipe(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	root = state.folderRoots[0]
	original := state.space.Current()
	original.Editor.Insert("unsaved draft")
	state.folderExpanded[root] = true
	if err := os.Rename(root, root+"-parked"); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(root, 0600); err != nil {
		t.Fatal(err)
	}
	state.refreshFolderTree()
	done := make(chan []folderRow, 1)
	go func() { done <- state.folderRows() }()
	select {
	case rows := <-done:
		t.Cleanup(state.stopFolderWatching)
		if len(rows) != 1 || state.folderWatcher != nil || !strings.Contains(state.notice, "Folder unavailable") {
			t.Fatalf("refresh did not reject replacement: rows=%+v notice=%q", rows, state.notice)
		}
		if state.space.Current() != original || original.Editor.String() != "unsaved draft" || !original.Dirty() {
			t.Fatal("refused refresh changed the open draft")
		}
	case <-time.After(2 * time.Second):
		// Release the old blocking open without writing to the pipe.
		if fd, err := unix.Open(root, unix.O_WRONLY|unix.O_NONBLOCK, 0); err == nil {
			_ = unix.Close(fd)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		}
		t.Fatal("folder refresh waited for a named pipe writer")
	}
}
