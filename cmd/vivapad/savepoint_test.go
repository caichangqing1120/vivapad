package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestEqualReplacementUsesDirtyCloseGateUntilUndoReachesSavePoint(t *testing.T) {
	state, driver := startDriver(t)
	path := filepath.Join(t.TempDir(), "equal.txt")
	if err := os.WriteFile(path, []byte("cat keep cat"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	state.query.Insert("cat")
	state.replacement.Insert("cat")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchID)
	clickID(t, driver, replaceAllID)
	if doc.Editor.String() != "cat keep cat" || !doc.Dirty() || !doc.Editor.CanUndo() {
		t.Fatal("equal Replace All was falsely clean")
	}
	state.closeTab(state.space.Active)
	if state.pendingClose < 0 || state.space.Current() != doc {
		t.Fatal("equal-target history bypassed dirty close confirmation")
	}
	state.cancelClose()
	state.command("Undo")
	if doc.Dirty() || !doc.Editor.CanRedo() {
		t.Fatal("Undo did not return the edited file to its saved boundary")
	}
	state.closeTab(state.space.Active)
	if state.pendingClose >= 0 || state.documentIndex(doc) >= 0 {
		t.Fatal("saved-boundary close still required a dirty decision")
	}
}

func TestSharedCloneSavePointDoesNotCleanAnotherDocument(t *testing.T) {
	state, driver := startDriver(t)
	root := t.TempDir()
	first := state.space.Current()
	first.Editor.Insert("cat keep cat")
	if err := first.Save(filepath.Join(root, "first.txt")); err != nil {
		t.Fatal(err)
	}
	second := state.space.NewDocument()
	second.Editor.Insert("other")
	if err := second.Save(filepath.Join(root, "second.txt")); err != nil {
		t.Fatal(err)
	}
	second.Editor.Insert("!")
	second.Editor.DeleteBackward()
	state.space.Select(state.documentIndex(first))
	state.showInActivePane(first)
	state.command("Clone to Other View")
	first.Editor.SetSelection(text.Range{Start: 0, End: 3})
	first.Editor.Insert("cat")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !first.Dirty() || !second.Dirty() || state.paneDocs[0] != first || state.paneDocs[1] != first || state.activePane != 1 {
		t.Fatal("equal clone edit lost independent source save points")
	}
	clickID(t, driver, undoID)
	if first.Dirty() || !second.Dirty() {
		t.Fatal("clone Undo cleaned the wrong source")
	}
	clickID(t, driver, redoID)
	state.command("Save")
	if first.Dirty() || !second.Dirty() || !first.Editor.CanUndo() {
		t.Fatal("clone Save changed another point or erased history")
	}
	clickID(t, driver, undoID)
	if !first.Dirty() || !second.Dirty() || first.Editor.String() != "cat keep cat" {
		t.Fatal("Undo before the newly saved equal-text unit was falsely clean")
	}
}

func TestCaptureSavePointStatesWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_SAVEPOINT_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_SAVEPOINT_SHOTS to capture saved/dirty Undo boundaries")
	}
	path := filepath.Join(t.TempDir(), "saved-point.txt")
	if err := os.WriteFile(path, []byte("cat keep cat"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	state.query.Insert("cat")
	state.replacement.Insert("cat")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	capture := func(name string, width, height int) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s save-point layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "savepoint-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	capture("clean-desktop", 1280, 800)
	clickID(t, session.Driver(), searchID)
	clickID(t, session.Driver(), replaceAllID)
	if !doc.Dirty() || !doc.Editor.CanUndo() {
		t.Fatal("equal-target capture did not become modified")
	}
	state.searchOpen = false
	capture("equal-dirty-compact", 640, 400)
	state.closeTab(state.space.Active)
	if state.pendingClose < 0 {
		t.Fatal("equal-target capture skipped dirty close")
	}
	capture("equal-close-compact", 640, 400)
	state.cancelClose()
	state.command("Undo")
	if doc.Dirty() {
		t.Fatal("save-point capture did not return to saved state")
	}
	capture("undo-clean-compact", 640, 400)
}
