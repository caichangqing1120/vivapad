package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/session"
)

func TestDocumentListActivateAndDirtyCloseUseExistingGate(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Name = "first.txt"
	first.Editor.Insert("first")
	second := state.space.NewDocument()
	second.Name = "second.txt"
	second.Editor.Insert("keep")
	state.pinned[first] = true
	state.space.Select(state.documentIndex(first))
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(19))
	if !state.documentListVisible {
		t.Fatal("View command did not show Document List")
	}
	foundPinned, foundDirty := false, false
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Open first.txt" && strings.Contains(hit.Hint, "Pinned") {
			foundPinned = true
		}
		if hit.Label == "Open second.txt" && strings.Contains(hit.Hint, "Modified") {
			foundDirty = true
		}
	}
	if !foundPinned || !foundDirty {
		t.Fatalf("document status invisible: pinned=%v dirty=%v", foundPinned, foundDirty)
	}
	clickLabeledControl(t, driver, "Open second.txt")
	if state.space.Current() != second || second.Editor.String() != "keep" {
		t.Fatal("Document List activation selected the wrong tab")
	}
	clickLabeledControl(t, driver, "Close second.txt")
	if state.pendingClose < 0 || state.documentIndex(second) < 0 {
		t.Fatal("Document List close bypassed dirty confirmation")
	}
	clickID(t, driver, cancelCloseID)
	if state.documentIndex(second) < 0 || second.Editor.String() != "keep" || !second.Dirty() {
		t.Fatal("canceled Document List close lost dirty text")
	}
}

func TestDocumentListSplitGroupsAndCloneCloseLeavePeerViewUntouched(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Name = "left.txt"
	left.Editor.Insert("shared draft")
	right := state.space.NewDocument()
	right.Name = "right.txt"
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 0
	state.space.Select(state.documentIndex(left))
	state.command("Document List")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	seenMain, seenSub := false, false
	for _, label := range driver.Snapshot().Texts {
		seenMain = seenMain || label.Value == "Main View"
		seenSub = seenSub || label.Value == "Sub View"
	}
	if !seenMain || !seenSub {
		t.Fatal("split documents were not grouped by view")
	}
	clickLabeledControl(t, driver, "Open right.txt")
	if state.activePane != 1 || state.space.Current() != right || state.paneDocs[0] != left {
		t.Fatal("Document List activation changed the peer pane")
	}
	state.paneDocs[1] = left
	state.space.Select(state.documentIndex(left))
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickLabeledControl(t, driver, "Close left.txt from sub view")
	if state.pendingClose >= 0 || state.documentIndex(left) < 0 || state.paneDocs[0] != left || left.Editor.String() != "shared draft" {
		t.Fatal("closing shared clone removed its owner or prompted for dirty data")
	}
}

func TestDocumentListCoexistsWithOtherDocksAndMapAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	state.folderVisible = true
	state.functionListVisible = true
	state.documentMapVisible = true
	state.command("Document List")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	for _, id := range []ui.ID{leftDockDocumentsID, leftDockWorkspaceID, leftDockFunctionsID, documentMapID} {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			found = found || hit.ID == id
			if hit.ID == id && (hit.Bounds.X < 0 || hit.Bounds.Y < menuHeight || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight) {
				t.Fatalf("dock control %v clipped: %+v", id, hit.Bounds)
			}
		}
		if !found {
			t.Fatalf("dock control %v missing", id)
		}
	}
	clickID(t, driver, leftDockWorkspaceID)
	clickID(t, driver, leftDockFunctionsID)
	clickID(t, driver, leftDockDocumentsID)
	clickID(t, driver, documentListCloseID)
	if state.documentListVisible || !state.folderVisible || !state.functionListVisible || !state.documentMapVisible || state.activeLeftDock != "workspace" {
		t.Fatal("closing Document List lost another dock")
	}
}

func TestDocumentListVisibilityAndActiveDockSurviveSessionRestart(t *testing.T) {
	root := t.TempDir()
	path, sessionPath := filepath.Join(root, "source.txt"), filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, sessionPath)
	state.folderVisible, state.functionListVisible = true, true
	state.command("Document List")
	newConfig(state).OnCloseRequest(func() {})
	stored, err := session.Load(sessionPath)
	if err != nil || !stored.DocumentListVisible || stored.ActiveLeftDock != "documents" {
		t.Fatalf("saved Document List = %+v, %v", stored, err)
	}
	for _, paths := range [][]string{nil, {path}} {
		restored := newStartupModel(paths, sessionPath)
		if !restored.documentListVisible || !restored.folderVisible || !restored.functionListVisible || restored.activeLeftDock != "documents" {
			t.Fatalf("restored Document List with args=%v: visible=%v dock=%q", paths, restored.documentListVisible, restored.activeLeftDock)
		}
	}
}

func TestDocumentListLongListScrollAndLiveStatus(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Name = "first.txt"
	for index := range 32 {
		doc := state.space.NewDocument()
		doc.Name = "tab" + strconv.Itoa(index) + ".txt"
	}
	state.space.Select(state.documentIndex(first))
	state.command("Document List")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if state.documentListVisibleRows >= len(state.documentListRows) || state.documentListVisibleRows < 1 {
		t.Fatalf("unbounded Document List rows: %d of %d", state.documentListVisibleRows, len(state.documentListRows))
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID.Scope() != documentListRowScope && hit.ID.Scope() != documentListRowCloseScope {
			continue
		}
		if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight || hit.Bounds.X+hit.Bounds.Width > 640 {
			t.Fatalf("row clipped: %+v", hit.Bounds)
		}
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID.Scope() == documentListRowScope {
			if err := driver.Scroll(ui.ScrollEvent{X: hit.Bounds.X + 10, Y: hit.Bounds.Y + 8, DeltaY: 120}); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if state.documentListOffset == 0 {
		t.Fatal("Document List ignored wheel input")
	}
	lastLabel := "Open tab31.txt"
	for step := 0; step < 40; step++ {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			found = found || hit.Label == lastLabel
		}
		if found {
			break
		}
		clickID(t, driver, documentListNextID)
	}
	clickLabeledControl(t, driver, lastLabel)
	if state.space.Current().Name != "tab31.txt" || state.space.Current().Dirty() {
		t.Fatal("last document did not activate cleanly")
	}
	state.space.Current().Editor.Insert("draft")
	state.pinned[state.space.Current()] = true
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == lastLabel && strings.Contains(hit.Hint, "Modified") && strings.Contains(hit.Hint, "Pinned") {
			found = true
		}
	}
	if !found {
		t.Fatal("visible Document List row did not refresh dirty/pin status")
	}
}

func TestCaptureDocumentListWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_DOCUMENT_LIST_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_DOCUMENT_LIST_SHOTS to capture Document List UI")
	}
	state := newModel()
	left := state.space.Current()
	left.Name = "notes.go"
	left.Editor.Insert("package main\n\nfunc main() {\n    println(\"hello\")\n}\n")
	right := state.space.NewDocument()
	right.Name = "settings.json"
	right.Editor.Insert("{\n  \"theme\": \"light\"\n}\n")
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 0
	state.space.Select(state.documentIndex(left))
	state.pinned[left] = true
	state.folderVisible, state.functionListVisible, state.documentMapVisible = true, true, true
	state.command("Document List")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 800},
		{"compact", 640, 400},
	} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "document-list-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}
