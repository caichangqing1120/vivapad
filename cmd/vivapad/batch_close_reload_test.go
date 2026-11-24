package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloseAllClosesMonitoredTabReloadedDuringConfirmation(t *testing.T) {
	root := t.TempDir()
	draftPath, monitorPath := filepath.Join(root, "draft.txt"), filepath.Join(root, "tail.log")
	for _, path := range []string{draftPath, monitorPath} {
		if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{draftPath, monitorPath})
	draft, monitored := state.space.Documents[0], state.space.Documents[1]
	draft.Editor.Insert("unsaved")
	state.space.Select(1)
	state.paneDocs[0] = monitored
	state.toggleMonitoring()
	t.Cleanup(state.stopAllMonitors)
	watcher := state.monitors[monitored]
	if watcher == nil {
		t.Fatalf("fixture did not start monitoring: %s", state.notice)
	}
	state.startBatchClose(nil)
	if !state.batchActive || state.pendingClose != 0 || len(state.space.Documents) != 2 {
		t.Fatal("close did not wait at the unsaved tab")
	}
	if err := os.WriteFile(monitorPath, []byte("reloaded\n"), 0600); err != nil {
		t.Fatal(err)
	}
	watcher.update.Store(monitorFileChanged)
	state.consumeMonitorChanges()
	if updated := state.space.Documents[1]; updated == monitored || updated.Editor.String() != "reloaded\n" || state.monitors[updated] != watcher {
		t.Fatal("fixture did not reload the monitored tab during confirmation")
	}
	state.finishClose(true)
	if state.batchActive || state.pendingClose >= 0 || len(state.space.Documents) != 1 || state.space.Current().Path != "" || len(state.monitors) != 0 {
		t.Fatalf("Close All left the reloaded tab open: current=%q; %s", state.space.Current().Path, state.notice)
	}
	if data, err := os.ReadFile(draftPath); err != nil || string(data) != "original\n" {
		t.Fatalf("discard changed the saved draft: %q, %v", data, err)
	}
}
