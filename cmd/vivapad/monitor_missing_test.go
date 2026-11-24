package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"vivapad/internal/document"
)

func TestMonitoringRechecksMissingPathBeforeStopping(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.log")
	if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	original := state.space.Current()
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement\n"), 0600); err != nil {
		t.Fatal(err)
	}
	watcher, err := newFileMonitor(original.Path, func() {}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = watcher.Close() })
	state.monitors = map[*document.Document]*fileMonitor{original: watcher}
	// The editor receives the older observation before the next watcher poll.
	watcher.update.Store(monitorPathMissing)
	state.consumeMonitorChanges()
	updated := state.space.Current()
	if updated.Editor.String() != "replacement\n" || state.monitors[updated] != watcher {
		t.Fatalf("a stale missing notification stopped monitoring: text=%q notice=%q", updated.Editor.String(), state.notice)
	}
}

func TestFileMonitorReportsContinuedAbsenceAfterConsume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.log")
	if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	wake := make(chan struct{}, 1)
	watcher, err := newFileMonitor(path, func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}, 25*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = watcher.Close() })
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	waitMonitorEvent(t, watcher, wake, true)
	// The editor may have observed a brief return between polls. Consuming the
	// first notification must not hide continued absence from the next refresh.
	waitMonitorEvent(t, watcher, wake, true)
}
