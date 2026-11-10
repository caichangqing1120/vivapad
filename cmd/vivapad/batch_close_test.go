package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/document"
)

func TestCloseAllCleanTabsLeavesOneFreshDocument(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{first, second})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(11))
	if len(state.space.Documents) != 1 || state.space.Current().Path != "" || state.space.Current().Dirty() || state.batchActive {
		t.Fatalf("Close All = %+v active=%v", state.space.Documents, state.batchActive)
	}
}

func TestCloseOthersSaveCancelAndDiscardAreSequential(t *testing.T) {
	state := newModel()
	first := state.space.Current()
	first.Editor.Insert("first")
	second := state.space.NewDocument()
	second.Editor.Insert("second")
	protected := state.space.NewDocument()
	state.showInActivePane(protected)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(12))
	if !state.batchActive || state.pendingClose != 0 || len(state.space.Documents) != 3 {
		t.Fatalf("first batch prompt = pending:%d docs:%d", state.pendingClose, len(state.space.Documents))
	}
	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveCloseID)
	if state.pendingClose != 0 || !first.Dirty() || len(state.space.Documents) != 3 {
		t.Fatal("canceled Save As skipped a dirty tab")
	}
	clickID(t, driver, cancelCloseID)
	if state.batchActive || state.pendingClose != -1 || len(state.space.Documents) != 3 || !second.Dirty() {
		t.Fatal("batch Cancel discarded an unconfirmed document")
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(12))
	path := filepath.Join(t.TempDir(), "first.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveCloseID)
	if state.pendingClose != 0 || len(state.space.Documents) != 2 {
		t.Fatalf("second dirty tab was not prompted: pending=%d docs=%d", state.pendingClose, len(state.space.Documents))
	}
	clickID(t, driver, discardID)
	if state.batchActive || state.pendingClose != -1 || len(state.space.Documents) != 1 || state.space.Current() != protected {
		t.Fatalf("Close Others = %+v pending=%d", state.space.Documents, state.pendingClose)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "first" {
		t.Fatalf("saved first tab = %q, %v", data, err)
	}
}

func TestCloseAllCancelAfterPartialProgressLeavesRemainingTabs(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "clean.txt")
	if err := os.WriteFile(path, []byte("clean"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	first := state.space.Current()
	second := state.space.NewDocument()
	second.Editor.Insert("keep")
	third := state.space.NewDocument()
	state.showInActivePane(third)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(11))
	if state.pendingClose != 0 || len(state.space.Documents) != 2 || state.documentIndex(first) >= 0 {
		t.Fatalf("partial close = pending:%d docs:%+v", state.pendingClose, state.space.Documents)
	}
	clickID(t, driver, cancelCloseID)
	if state.batchActive || len(state.space.Documents) != 2 || state.space.Documents[0] != second || state.space.Documents[1] != third ||
		!strings.Contains(state.notice, "canceled after 1") {
		t.Fatalf("cancel after partial close = %+v notice=%q", state.space.Documents, state.notice)
	}
}

func TestCloseOthersKeepsActiveSplitPaneBound(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "left.txt"), filepath.Join(root, "right.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles(paths)
	left, right := state.space.Documents[0], state.space.Documents[1]
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(1)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(12))
	if state.space.Current() != right || state.paneDocs[0] != right || state.paneDocs[1] != right || state.paneOwner[right] != 1 || len(state.space.Documents) != 1 {
		t.Fatalf("split Close Others = current:%p panes:%+v owners:%+v", state.space.Current(), state.paneDocs, state.paneOwner)
	}
}

func TestWindowCloseWaitsForBatchDirtyDecision(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("unsaved")
	state.space.NewDocument()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(11))
	if !state.batchActive || state.pendingClose != 0 {
		t.Fatalf("missing batch confirmation: pending=%d active=%v", state.pendingClose, state.batchActive)
	}
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	if allowed || !state.batchActive || state.pendingClose != 0 {
		t.Fatal("window close bypassed the batch Save/Discard/Cancel gate")
	}
	clickID(t, driver, cancelCloseID)
	if len(state.space.Documents) != 2 || !state.space.Documents[0].Dirty() {
		t.Fatal("canceling batch after close request lost unsaved content")
	}
}
