package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
)

func TestRectangleReloadSameTextRebindsBothCloneViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.txt")
	if err := os.WriteFile(path, []byte("abc\nxyz"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.command("Clone to Other View")
	for pane := range 2 {
		state.activatePane(pane)
		doc.Editor.MoveTo(1, false)
		state.command("Begin/End Select in Column Mode")
		doc.Editor.MoveTo(7, false)
		state.command("Begin/End Select in Column Mode")
	}
	state.activatePane(1)
	state.rectangleTextInput(0)
	if !state.rectangles[0].copyValid {
		t.Fatal("rectangle copy cache was not populated before reload")
	}
	state.recordRectangleTransition(0, doc, "before", "after", state.rectangles[0])
	newDoc := state.reloadDocument(state.space.Active, false)
	if newDoc == nil || newDoc == doc || state.paneDocument(0) != newDoc || state.paneDocument(1) != newDoc {
		t.Fatalf("reload did not replace both clones: %+v, %q", state.paneDocs, state.notice)
	}
	for pane := range 2 {
		selection := state.rectangles[pane]
		if !selection.active || selection.doc != newDoc || selection.anchor != 1 || selection.caret != 7 || selection.value != newDoc.Editor.String() || selection.copyValid {
			t.Fatalf("pane %d selection after reload = %+v", pane, selection)
		}
		if len(state.rectangleHistory[pane]) != 0 || state.rectangleHistoryIndex[pane] != 0 {
			t.Fatalf("pane %d kept stale undo history", pane)
		}
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.activatePane(0)
	state.command("Copy")
	if driver.Clipboard() != "bc\nyz\n" {
		t.Fatalf("rebound rectangle copy = %q", driver.Clipboard())
	}
}

func TestRectangleReloadSameTextPreservesPendingAnchor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.txt")
	if err := os.WriteFile(path, []byte("abc\nxyz"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	newDoc := state.reloadDocument(state.space.Active, false)
	if newDoc == nil || !state.rectangles[0].pending || state.rectangles[0].doc != newDoc {
		t.Fatalf("pending selection after reload = %+v", state.rectangles[0])
	}
	newDoc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	if !state.rectangles[0].active || state.rectangles[0].anchor != 1 || state.rectangles[0].caret != 7 {
		t.Fatalf("pending selection did not finish: %+v", state.rectangles[0])
	}
}

func TestRectangleReloadChangedTextClearsStaleSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changed.txt")
	if err := os.WriteFile(path, []byte("abc\nxyz"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	if err := os.WriteFile(path, []byte("different\ntext"), 0600); err != nil {
		t.Fatal(err)
	}
	newDoc := state.reloadDocument(state.space.Active, false)
	if newDoc == nil || newDoc.Editor.String() != "different\ntext" || state.rectangles[0].doc != nil || state.rectangles[0].active {
		t.Fatalf("changed reload retained stale selection: doc=%v rect=%+v notice=%q", newDoc, state.rectangles[0], state.notice)
	}
}

func TestRectangleReloadOutsideSelectedLinesRebasesCloneOffsets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(path, []byte("猫\nabc\nxyz\nend"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.command("Clone to Other View")
	for pane := range 2 {
		state.activatePane(pane)
		doc.Editor.MoveTo(3, false)
		state.command("Begin/End Select in Column Mode")
		doc.Editor.MoveTo(8, false)
		state.command("Begin/End Select in Column Mode")
	}
	state.activatePane(1)
	if err := os.WriteFile(path, []byte("猫猫前\nabc\nxyz\ntail"), 0600); err != nil {
		t.Fatal(err)
	}
	current := state.reloadDocument(state.space.Active, false)
	if current == nil {
		t.Fatal(state.notice)
	}
	for pane := range 2 {
		selection := state.rectangles[pane]
		if !selection.active || selection.doc != current || selection.anchor != 5 || selection.caret != 10 || selection.value != current.Editor.String() || selection.copyValid {
			t.Fatalf("pane %d rectangle after outside edit = %+v", pane, selection)
		}
	}
	if current.Editor.Caret() != state.rectangles[state.activePane].caret {
		t.Fatalf("active caret %d did not follow reloaded rectangle %d", current.Editor.Caret(), state.rectangles[state.activePane].caret)
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.activatePane(0)
	state.command("Copy")
	if driver.Clipboard() != "b\ny\n" {
		t.Fatalf("rebased rectangle copied %q", driver.Clipboard())
	}
	state.activatePane(1)
	input := state.rectangleTextInput(1)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, Text: "Q"}); err != nil || current.Editor.String() != "猫猫前\naQc\nxQz\ntail" {
		t.Fatalf("typing after outside reload = %q, %v", current.Editor.String(), err)
	}
}

func TestRectangleReloadChangedTextDropsPendingAnchor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending-changed.txt")
	if err := os.WriteFile(path, []byte("first\nabc"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(8, false)
	if err := os.WriteFile(path, []byte("longer-first\nabc"), 0600); err != nil {
		t.Fatal(err)
	}
	if state.reloadDocument(state.space.Active, false) == nil || state.rectangles[0].doc != nil || state.rectangles[0].pending {
		t.Fatalf("changed reload kept pending anchor: %+v", state.rectangles[0])
	}
}

func TestRectangleReloadLineInsertionClearsSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inserted.txt")
	if err := os.WriteFile(path, []byte("first\nabc\nxyz"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(12, false)
	state.command("Begin/End Select in Column Mode")
	if err := os.WriteFile(path, []byte("first\nnew\nabc\nxyz"), 0600); err != nil {
		t.Fatal(err)
	}
	if state.reloadDocument(state.space.Active, false) == nil || state.rectangles[0].doc != nil {
		t.Fatalf("line insertion retained stale rectangle: %+v", state.rectangles[0])
	}
}

func TestRectangleReloadFailureKeepsExistingSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	if err := os.WriteFile(path, []byte("abc\nxyz"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := state.reloadDocument(state.space.Active, false); got != nil || state.space.Current() != doc || !state.rectangles[0].active || state.rectangles[0].doc != doc {
		t.Fatalf("failed reload changed selection: doc=%v rect=%+v notice=%q", got, state.rectangles[0], state.notice)
	}
}

func TestCaptureReloadedCloneRectangleWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RECTANGLE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RECTANGLE_SHOTS to capture reloaded clone")
	}
	path := filepath.Join(t.TempDir(), "shared.txt")
	if err := os.WriteFile(path, []byte("abc\nxyz"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.command("Clone to Other View")
	state.activatePane(0)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	state.activatePane(1)
	if state.reloadDocument(state.space.Active, false) == nil {
		t.Fatal(state.notice)
	}
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("reloaded clone layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "column-selection-clone-reload-compact.png")); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRebasedCloneRectangleWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RECTANGLE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RECTANGLE_SHOTS to capture rebased clone")
	}
	path := filepath.Join(t.TempDir(), "external.txt")
	if err := os.WriteFile(path, []byte("猫\nabc\nxyz\nend"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.command("Clone to Other View")
	state.activatePane(0)
	doc.Editor.MoveTo(3, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(8, false)
	state.command("Begin/End Select in Column Mode")
	state.activatePane(1)
	if err := os.WriteFile(path, []byte("猫猫前\nabc\nxyz\ntail"), 0600); err != nil {
		t.Fatal(err)
	}
	if state.reloadDocument(state.space.Active, false) == nil {
		t.Fatal(state.notice)
	}
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("rebased clone layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "column-selection-clone-external-compact.png")); err != nil {
		t.Fatal(err)
	}
}
