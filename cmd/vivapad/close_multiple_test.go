package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func TestCloseToLeftUsesCurrentViewOnly(t *testing.T) {
	root := t.TempDir()
	paths := []string{"left-one.txt", "right-one.txt", "left-two.txt", "right-two.txt", "left-active.txt"}
	for index, name := range paths {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		paths[index] = path
	}
	state := newModelWithFiles(paths)
	leftOne, rightOne, leftTwo, rightTwo, active := state.space.Documents[0], state.space.Documents[1], state.space.Documents[2], state.space.Documents[3], state.space.Documents[4]
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{active, rightTwo}
	state.paneOwner[leftOne], state.paneOwner[leftTwo], state.paneOwner[active] = 0, 0, 0
	state.paneOwner[rightOne], state.paneOwner[rightTwo] = 1, 1
	state.activePane = 0
	state.space.Select(4)
	state.command("Close All to the Left")
	if len(state.space.Documents) != 3 || state.documentIndex(leftOne) >= 0 || state.documentIndex(leftTwo) >= 0 ||
		state.documentIndex(rightOne) < 0 || state.documentIndex(rightTwo) < 0 || state.space.Current() != active || state.paneDocs[1] != rightTwo {
		t.Fatalf("current-view left close = %+v, panes=%+v", state.space.Documents, state.paneDocs)
	}
}

func TestCloseToLeftPromptsFromNearestTabAndCancelLeavesUnprocessed(t *testing.T) {
	root := t.TempDir()
	cleanPath := filepath.Join(root, "clean.txt")
	if err := os.WriteFile(cleanPath, []byte("clean"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{cleanPath})
	clean := state.space.Current()
	dirty := state.space.NewDocument()
	dirty.Editor.Insert("dirty")
	active := state.space.NewDocument()
	right := state.space.NewDocument()
	state.space.Select(2)
	state.showInActivePane(active)
	state.command("Close All to the Left")
	if state.pendingClose != 1 || state.space.Documents[0] != clean || len(state.space.Documents) != 4 {
		t.Fatalf("nearest dirty tab did not prompt first: pending=%d docs=%+v", state.pendingClose, state.space.Documents)
	}
	state.cancelClose()
	if state.batchActive || state.documentIndex(clean) < 0 || state.documentIndex(dirty) < 0 || !strings.Contains(state.notice, "canceled after 0") {
		t.Fatalf("cancel closed unprocessed tabs: %+v, notice=%q", state.space.Documents, state.notice)
	}
	state.command("Close All to the Left")
	state.finishClose(true)
	if state.batchActive || len(state.space.Documents) != 2 || state.space.Current() != active || state.documentIndex(right) < 0 || state.documentIndex(clean) >= 0 {
		t.Fatalf("confirmed left close = %+v, active=%p", state.space.Documents, state.space.Current())
	}
}

func TestCloseAllUnchangedSkipsDirtyTextAndFormatOnlyTabs(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "clean.txt"), filepath.Join(root, "format.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles(paths)
	clean, format := state.space.Documents[0], state.space.Documents[1]
	format.SetUTF8BOM(true)
	dirty := state.space.NewDocument()
	dirty.Editor.Insert("draft")
	state.space.Select(2)
	state.command("Close All Unchanged")
	if len(state.space.Documents) != 2 || state.documentIndex(clean) >= 0 || state.documentIndex(format) < 0 || state.documentIndex(dirty) < 0 || state.pendingClose >= 0 {
		t.Fatalf("clean-only close = %+v, pending=%d", state.space.Documents, state.pendingClose)
	}
}

func TestCloseToLeftDetachesCloneWithoutDeletingDocument(t *testing.T) {
	state := newModel()
	clone := state.space.Current()
	clone.Editor.Insert("unsaved clone")
	active := state.space.NewDocument()
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{active, clone}
	state.paneOwner[clone], state.paneOwner[active] = 0, 0
	state.activePane = 0
	state.space.Select(1)
	state.command("Close All to the Left")
	if len(state.space.Documents) != 2 || state.paneOwner[clone] != 1 || state.paneDocs[1] != clone || state.paneDocs[0] != active ||
		state.pendingClose >= 0 || !clone.Dirty() {
		t.Fatalf("cloned document was deleted or prompted: docs=%+v panes=%+v pending=%d", state.space.Documents, state.paneDocs, state.pendingClose)
	}
}

func TestCloseAllUnchangedDoesNotReattachActiveClone(t *testing.T) {
	state := newModel()
	clone := state.space.Current()
	dirty := state.space.NewDocument()
	dirty.Editor.Insert("keep dirty")
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{clone, clone}
	state.paneOwner[clone], state.paneOwner[dirty] = 0, 0
	state.activePane = 0
	state.space.Select(0)
	state.command("Close All Unchanged")
	if len(state.space.Documents) != 2 || state.paneDocs[0] != dirty || state.paneDocs[1] != clone || state.paneOwner[clone] != 1 ||
		state.space.Current() != dirty || !dirty.Dirty() {
		t.Fatalf("clean clone reattached to left view: docs=%+v panes=%+v active=%p", state.space.Documents, state.paneDocs, state.space.Current())
	}
}

func TestCloseToRightPromptsFarthestDirtyFirstAndReportsPartialCancel(t *testing.T) {
	state := newModel()
	active := state.space.Current()
	near := state.space.NewDocument()
	near.Editor.Insert("near")
	clean := state.space.NewDocument()
	far := state.space.NewDocument()
	far.Editor.Insert("far")
	state.space.Select(0)
	state.showInActivePane(active)
	state.command("Close All to the Right")
	if state.pendingClose != 3 || state.space.Current() != active {
		t.Fatalf("right close did not prompt farthest dirty tab first: pending=%d", state.pendingClose)
	}
	state.finishClose(true)
	if state.pendingClose != 1 || state.documentIndex(far) >= 0 || state.documentIndex(clean) >= 0 || state.documentIndex(near) < 0 {
		t.Fatalf("right batch did not advance from farthest: pending=%d docs=%+v", state.pendingClose, state.space.Documents)
	}
	state.cancelClose()
	if state.batchActive || state.documentIndex(near) < 0 || !strings.Contains(state.notice, "canceled after 2") {
		t.Fatalf("partial cancel lost near tab: docs=%+v notice=%q", state.space.Documents, state.notice)
	}
}

func TestCloseMultipleSubmenuFitsCompactViewport(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(15))
	for index := range closeMultipleCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == closeMultipleScope.At(index) {
				found = true
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("close submenu row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("close submenu row %d missing", index)
		}
	}
	state.space.NewDocument()
	clickID(t, driver, closeMultipleScope.At(0))
	if len(state.space.Documents) != 1 || state.batchActive {
		t.Fatalf("close-but-active menu = %+v, batch=%v", state.space.Documents, state.batchActive)
	}
}

func TestCloseMultipleMenuUnchangedKeepsDirtyActiveTab(t *testing.T) {
	state, driver := startDriver(t)
	dirty := state.space.Current()
	dirty.Editor.Insert("keep")
	clean := state.space.NewDocument()
	state.space.Select(0)
	state.showInActivePane(dirty)
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(15))
	clickID(t, driver, closeMultipleScope.At(4))
	if state.space.Current() != dirty || state.documentIndex(clean) >= 0 || state.pendingClose >= 0 || !dirty.Dirty() {
		t.Fatalf("menu closed dirty active tab: docs=%+v, pending=%d", state.space.Documents, state.pendingClose)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(15))
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.closeMultipleMenuOpen || state.menuOpen != 0 {
		t.Fatalf("Escape did not return to parent File menu: open=%v parent=%d", state.closeMultipleMenuOpen, state.menuOpen)
	}
}

func TestCloseAllUnchangedDoesNotTouchOtherView(t *testing.T) {
	state := newModel()
	leftClean := state.space.Current()
	rightClean := state.space.NewDocument()
	leftDirty := state.space.NewDocument()
	leftDirty.Editor.Insert("keep")
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{leftDirty, rightClean}
	state.paneOwner[leftClean], state.paneOwner[leftDirty] = 0, 0
	state.paneOwner[rightClean] = 1
	state.activePane = 0
	state.space.Select(2)
	state.command("Close All Unchanged")
	if state.documentIndex(leftClean) >= 0 || state.documentIndex(rightClean) < 0 || state.documentIndex(leftDirty) < 0 ||
		state.paneDocs[1] != rightClean || state.space.Current() != leftDirty {
		t.Fatalf("unchanged close crossed views: docs=%+v panes=%+v", state.space.Documents, state.paneDocs)
	}
}

func TestCloseToLeftKeepsActiveCloneOwnedByOtherView(t *testing.T) {
	state := newModel()
	leftOwned := state.space.Current()
	cloned := state.space.NewDocument()
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{cloned, cloned}
	state.paneOwner[leftOwned], state.paneOwner[cloned] = 0, 1
	state.activePane = 0
	state.space.Select(1)
	state.command("Close All to the Left")
	if len(state.space.Documents) != 1 || state.space.Current() != cloned || state.paneDocs[0] != cloned || state.paneDocs[1] != cloned ||
		state.paneOwner[cloned] != 1 {
		t.Fatalf("active clone ownership changed: docs=%+v panes=%+v owner=%d", state.space.Documents, state.paneDocs, state.paneOwner[cloned])
	}
}
