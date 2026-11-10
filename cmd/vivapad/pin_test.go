package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vivapad/internal/document"
	"vivapad/internal/recovery"
	"vivapad/internal/session"
)

func TestPinTabMovesToBoundaryAndKeepsSelection(t *testing.T) {
	state := newModel()
	first := state.space.Current()
	second := state.space.NewDocument()
	third := state.space.NewDocument()
	fourth := state.space.NewDocument()
	state.space.Select(1)
	state.showInActivePane(second)
	state.command("Pin Tab")
	if !state.isPinned(second) || state.space.Documents[0] != second || state.space.Current() != second {
		t.Fatalf("first pin = %+v, active=%p", state.space.Documents, state.space.Current())
	}
	state.space.Select(state.documentIndex(fourth))
	state.showInActivePane(fourth)
	state.command("Pin Tab")
	if state.space.Documents[0] != second || state.space.Documents[1] != fourth || state.space.Current() != fourth {
		t.Fatalf("second pin = %+v", state.space.Documents)
	}
	state.space.Select(state.documentIndex(second))
	state.showInActivePane(second)
	state.command("Unpin Tab")
	if state.isPinned(second) || state.space.Documents[0] != fourth || state.space.Documents[1] != second ||
		state.space.Documents[2] != first || state.space.Documents[3] != third || state.space.Current() != second {
		t.Fatalf("unpin boundary = %+v, active=%p", state.space.Documents, state.space.Current())
	}
}

func TestPinControlInEachViewSharesCloneFlag(t *testing.T) {
	state, driver := startDriver(t)
	clone := state.space.Current()
	other := state.space.NewDocument()
	state.space.Select(0)
	state.showInActivePane(clone)
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{clone, clone}
	state.paneOwner[clone], state.paneOwner[other] = 0, 1
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, leftPinScope.At(0))
	if !state.isPinned(clone) || state.paneTabs(0)[0] != clone || state.paneTabs(1)[0] != clone {
		t.Fatal("pin state or prefix not shared with cloned tab")
	}
	clickID(t, driver, rightPinScope.At(state.documentIndex(clone)))
	if state.isPinned(clone) || state.paneDocs[0] != clone || state.paneDocs[1] != clone {
		t.Fatal("unpinning cloned tab changed its view bindings")
	}
}

func TestCloseAllButPinnedVisitsBothViewsAndPreservesDirtyGate(t *testing.T) {
	state := newModel()
	leftPinned := state.space.Current()
	leftDirty := state.space.NewDocument()
	leftDirty.Editor.Insert("left draft")
	rightPinned := state.space.NewDocument()
	rightDirty := state.space.NewDocument()
	rightDirty.Editor.Insert("right draft")
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{leftDirty, rightDirty}
	state.paneOwner[leftPinned], state.paneOwner[leftDirty] = 0, 0
	state.paneOwner[rightPinned], state.paneOwner[rightDirty] = 1, 1
	state.pinned[leftPinned], state.pinned[rightPinned] = true, true
	state.activePane = 0
	state.space.Select(state.documentIndex(leftDirty))
	state.command("Close All but Pinned Documents")
	if state.pendingClose != state.documentIndex(leftDirty) || !state.batchActive {
		t.Fatalf("main-view dirty tab was not first: pending=%d", state.pendingClose)
	}
	state.finishClose(true)
	if state.pendingClose != state.documentIndex(rightDirty) || state.documentIndex(leftDirty) >= 0 {
		t.Fatalf("sub-view dirty tab was not next: pending=%d", state.pendingClose)
	}
	state.cancelClose()
	if state.documentIndex(leftPinned) < 0 || state.documentIndex(rightPinned) < 0 || state.documentIndex(rightDirty) < 0 ||
		!strings.Contains(state.notice, "canceled after 1") {
		t.Fatalf("partial cancel lost pinned/remaining tabs: %+v, %q", state.space.Documents, state.notice)
	}
}

func TestCloseAllButPinnedDetachesUnpinnedClone(t *testing.T) {
	state := newModel()
	clone := state.space.Current()
	clone.Editor.Insert("unsaved clone")
	protected := state.space.NewDocument()
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{clone, clone}
	state.paneOwner[clone], state.paneOwner[protected] = 1, 0
	state.pinned[protected] = true
	state.space.Select(0)
	state.command("Close All but Pinned Documents")
	if state.pendingClose != state.documentIndex(clone) || state.paneDocs[0] == clone || state.paneDocs[1] != clone {
		t.Fatalf("clone was not detached before its dirty prompt: pending=%d panes=%+v", state.pendingClose, state.paneDocs)
	}
	state.finishClose(true)
	if state.documentIndex(clone) >= 0 {
		t.Fatal("confirmed unpinned clone was not closed")
	}
	if state.documentIndex(protected) < 0 || !state.isPinned(protected) {
		t.Fatal("protected tab was closed")
	}
}

func TestPinButtonsFitCompactTabs(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, tabPinScope.At(0))
	if !state.isPinned(state.space.Current()) {
		t.Fatal("single-view pin button did not toggle")
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == tabPinScope.At(0) && (hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > menuHeight+toolbarHeight+tabHeight) {
			t.Fatalf("pin button is clipped: %+v", hit.Bounds)
		}
	}
}

func TestPinnedSavedAndDirtyDocumentsRestoreIndependently(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "saved.txt")
	if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{file}, privatePath)
	canonical := state.space.Current().Path
	state.command("Pin Tab")
	if !state.isPinned(state.space.Current()) {
		t.Fatalf("pin command did not set flag: docs=%+v panes=%+v", state.space.Documents, state.paneDocs)
	}
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	stored, err := session.Load(privatePath)
	if err != nil || !stored.Pinned[canonical] {
		t.Fatalf("pin flag not written to session: %+v, %v", stored, err)
	}
	reopened := newStartupModel(nil, privatePath)
	if !allowed || reopened.space.Current().Path != canonical || !reopened.isPinned(reopened.space.Current()) {
		t.Fatalf("saved pin was not restored: path=%q flags=%+v stored=%+v notice=%q allowed=%v", reopened.space.Current().Path, reopened.pinned, stored, reopened.notice, allowed)
	}
	draft := reopened.space.NewDocument()
	draft.Editor.Insert("draft")
	reopened.space.Select(reopened.documentIndex(draft))
	reopened.showInActivePane(draft)
	reopened.command("Pin Tab")
	if err := recovery.Save(reopened.recoveryPath, reopened.currentRecovery()); err != nil {
		t.Fatal(err)
	}
	recovered := newStartupModel(nil, privatePath)
	if len(recovered.space.Documents) != 2 || !recovered.isPinned(recovered.space.Documents[0]) ||
		!recovered.isPinned(recovered.space.Documents[1]) || recovered.space.Documents[1].Editor.String() != "draft" {
		t.Fatalf("dirty pin recovery = %+v, notice=%q", recovered.space.Documents, recovered.notice)
	}
}

func TestNPPExportUsesVisiblePinnedPrefixInBothViews(t *testing.T) {
	root := t.TempDir()
	paths := make([]string, 4)
	for index, name := range []string{"left-a.txt", "left-b.txt", "right-a.txt", "right-b.txt"} {
		paths[index] = filepath.Join(root, name)
		if err := os.WriteFile(paths[index], []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles(paths)
	leftA, leftB, rightA, rightB := state.space.Documents[0], state.space.Documents[1], state.space.Documents[2], state.space.Documents[3]
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{leftA, rightA}
	state.paneOwner[leftA], state.paneOwner[leftB] = 0, 0
	state.paneOwner[rightA], state.paneOwner[rightB] = 1, 1
	state.pinned[leftB], state.pinned[rightB] = true, true
	stored, err := state.nppSessionState()
	if err != nil || len(stored.Main) != 2 || len(stored.Sub) != 2 ||
		stored.Main[0].Path != leftB.Path || !stored.Main[0].Pinned || stored.Main[1].Pinned ||
		stored.Sub[0].Path != rightB.Path || !stored.Sub[0].Pinned || stored.Sub[1].Pinned {
		t.Fatalf("NPP tab order does not match visible views: %+v, %v", stored, err)
	}
}

func TestLoadNPPPinFlagAndConflictingClonePreflight(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "shared.txt")
	if err := os.WriteFile(file, []byte("shared"), 0600); err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	xmlPath := filepath.Join(root, "session.xml")
	state, driver := startDriver(t)
	dirty := state.space.Current()
	dirty.Editor.Insert("keep")
	if err := session.SaveNPP(xmlPath, session.NPPState{
		Main: []session.NPPFile{{Path: canonical, Encoding: -1, Pinned: true}},
		Sub:  []session.NPPFile{{Path: canonical, Encoding: -1}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Load Session...")
	if len(state.space.Documents) != 1 || state.space.Current() != dirty || !strings.Contains(state.notice, "conflicting document state") {
		t.Fatalf("conflicting clone pin changed tabs: %+v, notice=%q", state.space.Documents, state.notice)
	}
	if err := session.SaveNPP(xmlPath, session.NPPState{
		Main: []session.NPPFile{{Path: canonical, Encoding: -1, Pinned: true}},
		Sub:  []session.NPPFile{{Path: canonical, Encoding: -1, Pinned: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Load Session...")
	if len(state.space.Documents) != 2 || !state.splitEnabled || !state.isPinned(state.paneDocs[0]) ||
		state.paneDocs[0] != state.paneDocs[1] || !dirty.Dirty() {
		t.Fatalf("pinned clone import = %+v, panes=%+v, notice=%q", state.space.Documents, state.paneDocs, state.notice)
	}
}

func TestPinnedDocumentStaysPinnedAfterReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.txt")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.command("Pin Tab")
	previous := state.space.Current()
	if err := os.WriteFile(path, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	state.command("Reload from Disk")
	if current := state.space.Current(); current == previous || !state.isPinned(current) || state.isPinned(previous) || current.Editor.String() != "after" {
		t.Fatalf("reload pin transfer = %+v, oldPin=%v", state.space.Current(), state.isPinned(previous))
	}
}
