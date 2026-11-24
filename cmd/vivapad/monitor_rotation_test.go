package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"vivapad/internal/document"
)

func TestMonitoringReloadsFileRecreatedBeforeRefresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.log")
	if err := os.WriteFile(path, []byte("original\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	original := state.space.Current()
	wake := make(chan struct{}, 2)
	watcher, err := newFileMonitor(original.Path, func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}, 25*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = watcher.Close() })
	state.monitors = map[*document.Document]*fileMonitor{original: watcher}
	waitForWake := func(stage string) {
		t.Helper()
		select {
		case <-wake:
		case <-time.After(3 * time.Second):
			t.Fatalf("monitor did not observe %s", stage)
		}
	}
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	waitForWake("the missing path")
	if err := os.WriteFile(path, []byte("replacement\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	waitForWake("the replacement")
	// The UI consumes both observations after the monitor has seen the new file.
	state.consumeMonitorChanges()
	updated := state.space.Current()
	if updated.Editor.String() != "replacement\n" || state.monitors[updated] != watcher || updated.Dirty() {
		t.Fatalf("monitor failed to follow the replacement: text=%q notice=%q", updated.Editor.String(), state.notice)
	}
	if updated.LineEnding() != document.CRLF || updated.Editor.Caret() != updated.Editor.RuneCount() {
		t.Fatal("monitored reload did not preserve the line ending and follow the end")
	}
}
