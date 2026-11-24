package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowCloseRestoresTabStateAfterMonitoredReload(t *testing.T) {
	root := t.TempDir()
	draftPath, watchedPath := filepath.Join(root, "draft.txt"), filepath.Join(root, "tail.log")
	for _, path := range []string{draftPath, watchedPath} {
		if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{draftPath, watchedPath}, sessionPath)
	draft, watched := state.space.Documents[0], state.space.Documents[1]
	draft.Editor.Insert("unsaved")
	state.space.Select(state.documentIndex(watched))
	state.showInActivePane(watched)
	state.toggleSplit()
	state.command("Pin Tab")
	state.setTabColor("Apply Color 3")
	state.toggleMonitoring()
	t.Cleanup(state.stopAllMonitors)
	watcher := state.monitors[watched]
	if watcher == nil || !state.splitEnabled || state.activePane != 1 || state.paneDocument(0) != draft || state.paneDocument(1) != watched || state.paneOwner[watched] != 1 || !state.isPinned(watched) || state.tabColors[watched] != 3 {
		t.Fatalf("could not prepare the monitored right tab: %s", state.notice)
	}
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	if allowed || state.pendingClose != state.documentIndex(draft) {
		t.Fatal("window close did not wait for the unsaved tab")
	}
	if err := os.WriteFile(watchedPath, []byte("reloaded\n"), 0600); err != nil {
		t.Fatal(err)
	}
	watcher.update.Store(monitorFileChanged)
	state.consumeMonitorChanges()
	reloaded := state.paneDocument(1)
	if reloaded == watched || reloaded.Editor.String() != "reloaded\n" || state.monitors[reloaded] != watcher || !state.isPinned(reloaded) || state.tabColors[reloaded] != 3 {
		t.Fatal("monitored reload did not preserve the live right tab")
	}
	state.finishClose(true)
	if !allowed {
		t.Fatal("window close did not finish after discard")
	}
	reopened := newStartupModel(nil, sessionPath)
	right := reopened.paneDocument(1)
	if !reopened.splitEnabled || reopened.activePane != 1 || right.Path != watched.Path || reopened.paneOwner[right] != 1 || right.Editor.String() != "reloaded\n" {
		t.Fatalf("session lost the active right tab: split=%v active=%d owner=%d path=%q text=%q", reopened.splitEnabled, reopened.activePane, reopened.paneOwner[right], right.Path, right.Editor.String())
	}
	if !reopened.isPinned(right) || reopened.tabColors[right] != 3 {
		t.Fatalf("session lost the reloaded tab's pin or color: pinned=%v color=%d", reopened.isPinned(right), reopened.tabColors[right])
	}
	if reopened.isReadOnly(right) || len(reopened.monitors) != 0 {
		t.Fatal("session restored temporary monitoring as read-only")
	}
	if data, err := os.ReadFile(draftPath); err != nil || string(data) != "original\n" {
		t.Fatalf("discard changed the saved draft: %q, %v", data, err)
	}
}
