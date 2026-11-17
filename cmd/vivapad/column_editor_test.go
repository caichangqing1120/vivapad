package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestColumnEditorTextMenuAndOneUndo(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\nc\n")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 1, End: 1})
	state.setBookmarkLines(doc, []int{1})
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(22))
	if !state.columnDialogOpen {
		t.Fatal("Edit > Column Editor did not open")
	}
	clickID(t, driver, columnTextID)
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, columnApplyID)
	if state.columnDialogOpen || doc.Editor.String() != "aXb\ncX\n X" {
		t.Fatalf("column insert = %q, open=%v", doc.Editor.String(), state.columnDialogOpen)
	}
	if marked := state.bookmarkLines(doc); len(marked) != 1 || marked[0] != 1 {
		t.Fatalf("column edit lost bookmark: %+v", marked)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\nc\n" || doc.Editor.CanUndo() {
		t.Fatalf("column insert undo = %q", doc.Editor.String())
	}
	if marked := state.bookmarkLines(doc); len(marked) != 1 || marked[0] != 1 {
		t.Fatalf("column undo lost bookmark: %+v", marked)
	}
}

func TestColumnEditorHexModeLeadingAndUppercase(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("a\nb\nc")
	doc.Editor.SetSelection(text.Range{Start: 1, End: 1})
	state.command("Column Editor...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, columnNumberModeID)
	state.columnInitial.SetSelection(text.Range{Start: 0, End: state.columnInitial.RuneCount()})
	state.columnInitial.Insert("15")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, columnBaseScope.At(1))
	clickID(t, driver, columnLeadingScope.At(1))
	clickID(t, driver, columnUppercaseID)
	clickID(t, driver, columnApplyID)
	if doc.Editor.String() != "a0F\nb10\nc11" {
		t.Fatalf("hex column insert = %q", doc.Editor.String())
	}
}

func TestColumnEditorNumberFailureCancelReadOnlyAndSplit(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	state.command("Split View")
	state.activatePane(1)
	right := state.space.NewDocument()
	state.showInActivePane(right)
	right.Editor.Insert("a\nb\nc")
	right.Editor.ClearHistory()
	right.Editor.SetSelection(text.Range{Start: 1, End: 1})
	state.command("Column Editor...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, columnNumberModeID)
	state.columnInitial.SetSelection(text.Range{Start: 0, End: state.columnInitial.RuneCount()})
	state.columnInitial.Insert("bad")
	clickID(t, driver, columnApplyID)
	if !state.columnDialogOpen || state.columnError == "" || right.Editor.String() != "a\nb\nc" {
		t.Fatal("invalid column number modified the document")
	}
	clickID(t, driver, columnInitialID)
	if _, err := driver.ShortcutChord(primary(), 'n'); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 2 {
		t.Fatal("Column Editor allowed a new-document shortcut")
	}
	dropped := filepath.Join(t.TempDir(), "drop.txt")
	if err := os.WriteFile(dropped, []byte("drop"), 0600); err != nil {
		t.Fatal(err)
	}
	newConfig(state).OnFileDrop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, Paths: []string{dropped}})
	if len(state.space.Documents) != 2 {
		t.Fatal("Column Editor accepted a file drop")
	}
	clickID(t, driver, columnCancelID)
	if state.columnDialogOpen || left.Editor.String() != "left" || right.Editor.String() != "a\nb\nc" {
		t.Fatal("Column Editor cancellation modified a split view")
	}
	state.command("Column Editor...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, columnNumberModeID)
	clickID(t, driver, columnApplyID)
	if right.Editor.String() != "a0\nb1\nc2" || left.Editor.String() != "left" {
		t.Fatalf("column numbers changed wrong pane: left=%q right=%q", left.Editor.String(), right.Editor.String())
	}
	state.readOnly[right] = true
	state.command("Column Editor...")
	if state.columnDialogOpen || right.Editor.String() != "a0\nb1\nc2" {
		t.Fatal("read-only document opened a mutation dialog")
	}
}

func TestCaptureColumnEditorWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_COLUMN_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_COLUMN_SHOTS to capture Column Editor")
	}
	state := newModel()
	state.command("Column Editor...")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if size.name == "compact" {
			clickID(t, session.Driver(), columnNumberModeID)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("Column Editor %s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "column-editor-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.Driver().Key(gfx.KeyEscape, 0); err != nil || state.columnDialogOpen {
		t.Fatalf("Column Editor Escape failed: %v", err)
	}
	clickID(t, session.Driver(), menuScope.At(1))
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("Edit menu layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "column-editor-menu-compact.png")); err != nil {
		t.Fatal(err)
	}
}
