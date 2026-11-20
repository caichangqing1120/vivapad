package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemDocumentsOpenInActivePaneAndReuseDirtyTab(t *testing.T) {
	state, driver := startDriver(t)
	draft := state.space.Current()
	draft.Editor.Insert("left draft")
	state.toggleSplit()
	state.activatePane(1)
	path := filepath.Join(t.TempDir(), "new \u6587\u6863.txt")
	const original = "first\r\nsecond"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	state.queueSystemDocument(path)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	opened := state.space.Current()
	if state.activePane != 1 || state.paneDocument(0) != draft || state.paneDocument(1) != opened || opened.Editor.String() != "first\nsecond" {
		t.Fatal("system request did not open in the active pane while keeping the left draft")
	}
	opened.Editor.Insert("unsaved ")
	edited := opened.Editor.String()
	state.queueSystemDocument(path)
	state.queueSystemDocument(filepath.Join(filepath.Dir(path), "missing.txt"))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 2 || state.space.Current() != opened || !opened.Dirty() || opened.Editor.String() != edited || draft.Editor.String() != "left draft" {
		t.Fatal("reopening the same file or failing to open another replaced an existing edit")
	}
	if len(state.recentEntries) != 1 || state.recentEntries[0].Path != opened.Path || !strings.Contains(state.notice, "missing.txt") {
		t.Fatalf("recent entries = %v, status = %q", state.recentEntries, state.notice)
	}
	if contents, err := os.ReadFile(path); err != nil || string(contents) != original {
		t.Fatalf("source bytes = %q, error = %v", contents, err)
	}
}

func TestSystemDocumentsWaitForDirtyCloseDecision(t *testing.T) {
	state, driver := startDriver(t)
	draft := state.space.Current()
	draft.Editor.Insert("keep this draft")
	state.closeTab(0)
	path := filepath.Join(t.TempDir(), "new document.txt")
	if err := os.WriteFile(path, []byte("opened"), 0600); err != nil {
		t.Fatal(err)
	}
	state.queueSystemDocument(path)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.pendingClose != 0 || state.space.Current() != draft || len(state.space.Documents) != 1 {
		t.Fatal("incoming document interrupted the close decision")
	}
	state.cancelClose()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 2 || state.space.Current().Editor.String() != "opened" || draft.Editor.String() != "keep this draft" {
		t.Fatal("canceling Close did not preserve the draft and open the queued document")
	}
}
