package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestSplitLinesUsesPaintedWidthAndOneUndoUnit(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	line := strings.Repeat("alpha beta gamma ", 9)
	before := line + "\nfooter"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	breaks := state.codeEditor().SoftWrapOffsets(nil)
	count := 0
	for _, offset := range breaks {
		if offset < len(line) {
			count++
		}
	}
	if count < 2 {
		t.Fatalf("test line did not wrap at painted width: %v", breaks)
	}
	state.command("Split Lines")
	if got := doc.Editor.String(); strings.Count(got, "\n") != 1+count || strings.ReplaceAll(got[:strings.LastIndex(got, "\n")], "\n", "") != line {
		t.Fatalf("split text does not match painted breaks: %q, breaks=%v", got, breaks)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("split Undo = %q", doc.Editor.String())
	}
}

func TestSplitLinesSelectionAndBookmarkShift(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	first := strings.Repeat("first word ", 8)
	second := strings.Repeat("second word ", 8)
	doc.Editor.Insert(first + "\n" + second + "\ntail")
	doc.GoToLine(3)
	state.command("Toggle Bookmark")
	start := len([]rune(first + "\n"))
	doc.Editor.SetSelection(text.Range{Start: start, End: start + len([]rune(second))})
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	breaks := state.codeEditor().SoftWrapOffsets(nil)
	expectedShift := 0
	for _, offset := range breaks {
		if offset > len(first) && offset < len(first)+1+len(second) {
			expectedShift++
		}
	}
	if expectedShift == 0 {
		t.Fatal("selected second line did not wrap")
	}
	state.command("Split Lines")
	if got := doc.Editor.String(); !strings.HasPrefix(got, first+"\n") || strings.Count(got, "\n") != 2+expectedShift {
		t.Fatalf("split crossed selected line: %q", got)
	}
	if marks := state.bookmarkLines(doc); len(marks) != 1 || marks[0] != 2+expectedShift {
		t.Fatalf("split bookmark = %v, expected line %d", marks, 2+expectedShift)
	}
}

func TestSplitLinesCompactMenuAndReadOnlyGate(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type(strings.Repeat("wide text ", 15)); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	for index := 0; index <= 12; index++ {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == lineScope.At(index) {
				found = true
				if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("line command %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("line command %d missing", index)
		}
	}
	clickID(t, driver, lineScope.At(12))
	if strings.Count(state.space.Current().Editor.String(), "\n") == 0 {
		t.Fatal("Split Lines menu made no visual-width breaks")
	}
	state.command("Read-Only on Current Document")
	before := state.space.Current().Editor.String()
	state.command("Split Lines")
	if state.space.Current().Editor.String() != before || !strings.Contains(state.notice, "Read-only") {
		t.Fatalf("read-only split changed text or notice: %q", state.notice)
	}
}

func TestSplitLinesUsesActiveSplitPaneWidth(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert(strings.Repeat("left word ", 12))
	right := state.space.NewDocument()
	right.Editor.Insert(strings.Repeat("right word ", 12))
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	breaks := state.codeEditorForPane(1).SoftWrapOffsets(nil)
	if len(breaks) == 0 {
		t.Fatal("right pane did not wrap")
	}
	state.command("Split Lines")
	if left.Editor.String() != strings.Repeat("left word ", 12) || strings.Count(right.Editor.String(), "\n") != len(breaks) {
		t.Fatalf("split crossed panes: left=%q right=%q breaks=%v", left.Editor.String(), right.Editor.String(), breaks)
	}
}
