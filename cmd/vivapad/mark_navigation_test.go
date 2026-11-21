package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestNavigateFindMarksFollowsSourceBoundariesAndWraps(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	value := "pre 猫 mid 猫 end 猫"
	doc.Editor.Insert(value)
	doc.Editor.ClearHistory()
	state.command("Mark...")
	setMarkQuery(state, "猫")
	state.markAll()
	if len(state.markedSpans) != 3 {
		t.Fatalf("setup marks = %+v", state.markedSpans)
	}
	doc.Editor.MoveTo(0, false)
	for _, index := range []int{0, 1, 2, 0} {
		state.navigateMarks(false)
		span := state.markedSpans[index]
		want := text.Range{Start: doc.Editor.RuneIndex(span.Start), End: doc.Editor.RuneIndex(span.End)}
		if got := doc.Editor.Selection(); got != want || doc.Editor.Caret() != want.End {
			t.Fatalf("next mark %d = %v caret %d, want %v", index, got, doc.Editor.Caret(), want)
		}
	}
	state.navigateMarks(true)
	first := state.markedSpans[0]
	if doc.Editor.Selection() != (text.Range{Start: doc.Editor.RuneIndex(first.Start), End: doc.Editor.RuneIndex(first.End)}) || doc.Editor.Caret() != doc.Editor.RuneIndex(first.Start) {
		t.Fatalf("previous from forward end did not reverse current span: %v caret %d", doc.Editor.Selection(), doc.Editor.Caret())
	}
	state.navigateMarks(true)
	last := state.markedSpans[2]
	if doc.Editor.Selection() != (text.Range{Start: doc.Editor.RuneIndex(last.Start), End: doc.Editor.RuneIndex(last.End)}) || doc.Editor.Caret() != doc.Editor.RuneIndex(last.Start) {
		t.Fatalf("previous did not wrap to last: %v caret %d", doc.Editor.Selection(), doc.Editor.Caret())
	}
	if doc.Editor.String() != value || doc.Editor.CanUndo() {
		t.Fatalf("navigation changed text or undo: %q", doc.Editor.String())
	}
	state.command("Clone to Other View")
	state.activatePane(1)
	doc.Editor.MoveTo(0, false)
	state.navigateMarks(false)
	if !state.splitEnabled || state.activePane != 1 || state.paneDocs[0] != doc || state.paneDocs[1] != doc ||
		doc.Editor.Selection() != (text.Range{Start: doc.Editor.RuneIndex(first.Start), End: doc.Editor.RuneIndex(first.End)}) {
		t.Fatalf("shared clone navigation changed view or selection: panes=%+v active=%d selection=%v", state.paneDocs, state.activePane, doc.Editor.Selection())
	}
}

func TestCompactSearchMenuFindMarkJumpAndEscape(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("pre cat then cat")
	state.command("Mark...")
	setMarkQuery(state, "cat")
	state.markAll()
	doc.Editor.MoveTo(0, false)
	clickID(t, driver, menuScope.At(2))
	state.searchMenuOffset = 11
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchCommandID(t, "Jump Down"))
	if !state.markJumpMenuOpen || state.markJumpPrevious {
		t.Fatalf("Jump Down submenu: open=%v previous=%v", state.markJumpMenuOpen, state.markJumpPrevious)
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact Jump Down layout: %+v", problems)
	}
	clickID(t, driver, markJumpScope.At(5))
	if state.markJumpMenuOpen || state.menuOpen >= 0 || doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("Jump Down result: submenu=%v menu=%d selection=%v", state.markJumpMenuOpen, state.menuOpen, doc.Editor.Selection())
	}
	clickID(t, driver, menuScope.At(2))
	state.searchMenuOffset = 11
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchCommandID(t, "Jump Up"))
	if !state.markJumpMenuOpen || !state.markJumpPrevious {
		t.Fatal("Jump Up submenu missing")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.markJumpMenuOpen || state.menuOpen != 2 {
		t.Fatalf("Escape did not dismiss only nested menu: %v submenu=%v menu=%d", err, state.markJumpMenuOpen, state.menuOpen)
	}
}

func TestNavigateFindMarksRejectsOtherDocumentsAndReplacedEditor(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	state.command("Mark...")
	setMarkQuery(state, "cat")
	state.markAll()
	other := state.space.NewDocument()
	state.showInActivePane(other)
	other.Editor.Insert("outside")
	other.Editor.MoveTo(2, false)
	state.navigateMarks(false)
	if other.Editor.Caret() != 2 || state.markedDocument != doc {
		t.Fatalf("other document navigated source marks: caret %d source %p", other.Editor.Caret(), state.markedDocument)
	}
	state.showInActivePane(doc)
	state.space.Select(0)
	doc.Editor = text.NewEditor("!cat cat")
	doc.Editor.MoveTo(1, false)
	before := doc.Editor.Selection()
	state.navigateMarks(false)
	if doc.Editor.Selection() != before || !strings.Contains(state.notice, "Marks cleared") || state.markedDocument != nil {
		t.Fatalf("stale spans navigated: selection %v notice %q marks %+v", doc.Editor.Selection(), state.notice, state.markedSpans)
	}
}

func TestCaptureMarkJumpMenusWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_MARK_JUMP_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_MARK_JUMP_SHOTS to capture Mark jump menus")
	}
	state := newModel()
	state.space.Current().Editor.Insert("cat cat\ndog cat")
	state.command("Mark...")
	setMarkQuery(state, "cat")
	state.markAll()
	state.markDialogOpen = false
	state.menuOpen, state.markJumpMenuOpen = 2, true
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
		menuOffset    int
	}{
		{"desktop", 1280, 800, 0}, {"compact", 640, 400, 11},
	} {
		state.searchMenuOffset = size.menuOffset
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s Mark jump layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "mark-jump-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}
