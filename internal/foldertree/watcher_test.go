package foldertree

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherReportsDirectoryEntryChangesAndCloses(t *testing.T) {
	root := t.TempDir()
	woken := make(chan struct{}, 1)
	watcher, err := NewWatcher(func() {
		select {
		case woken <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { watcher.Close() })
	if err := watcher.Add(root); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "created.txt")
	if err := os.WriteFile(file, []byte("created"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-woken:
	case <-time.After(5 * time.Second):
		t.Fatal("file creation did not wake the watcher")
	}
	changed, err := watcher.Consume()
	if err != nil || !changed {
		t.Fatalf("watcher change = %v, %v", changed, err)
	}
	if err := watcher.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := watcher.Close(); err != nil {
		t.Fatal(err)
	}
}
