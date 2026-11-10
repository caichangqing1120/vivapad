package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestOpenTargetReplaceKeepsStylesBookmarksCloneAndPerTabHistory(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Editor.Insert("cat keep cat\nanchor")
	first.Editor.ClearHistory()
	first.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(0, false)
	state.query.Insert("keep")
	state.command("Mark All")
	first.GoToLine(2)
	state.toggleBookmark()
	clickID(t, driver, newID)
	second := state.space.Current()
	second.Editor.Insert("cat stay cat\nanchor")
	second.Editor.ClearHistory()
	second.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(1, false)
	second.GoToLine(2)
	state.toggleBookmark()
	state.space.Select(0)
	state.showInActivePane(first)
	state.command("Clone to Other View")
	setMarkQuery(state, "cat")
	state.replacement.Insert("X")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchID)
	clickID(t, driver, replaceOpenID)
	if first.Editor.String() != "X keep X\nanchor" || second.Editor.String() != "X stay X\nanchor" ||
		!slices.Equal(state.styleSpans(first, 0), []text.Span{{Start: 2, End: 6}}) || !slices.Equal(state.styleSpans(second, 1), []text.Span{{Start: 2, End: 6}}) ||
		state.markedDocument != first || !slices.Equal(state.markedSpans, []text.Span{{Start: 2, End: 6}}) {
		t.Fatalf("cross-tab targets lost styles: %q / %q, %+v / %+v", first.Editor.String(), second.Editor.String(), state.styleSpans(first, 0), state.styleSpans(second, 1))
	}
	if state.activePane != 1 || state.paneDocs[0] != first || state.paneDocs[1] != first || !strings.Contains(state.notice, "4 matches") || !strings.Contains(state.notice, "2 open tabs") {
		t.Fatalf("clone source was duplicated or views changed: %+v active %d, %q", state.paneDocs, state.activePane, state.notice)
	}
	if !slices.Equal(state.bookmarkLines(first), []int{1}) || !slices.Equal(state.bookmarkLines(second), []int{1}) {
		t.Fatal("open target replace changed untouched bookmarks")
	}
	clickID(t, driver, undoID)
	if first.Editor.String() != "cat keep cat\nanchor" || first.Editor.CanUndo() || second.Editor.String() != "X stay X\nanchor" || !slices.Equal(state.styleSpans(first, 0), []text.Span{{Start: 4, End: 8}}) || !slices.Equal(state.markedSpans, []text.Span{{Start: 4, End: 8}}) {
		t.Fatal("active clone Undo changed other source or lost range")
	}
	clickID(t, driver, redoID)
	if !slices.Equal(state.styleSpans(first, 0), []text.Span{{Start: 2, End: 6}}) || !slices.Equal(state.markedSpans, []text.Span{{Start: 2, End: 6}}) || second.Editor.String() != "X stay X\nanchor" {
		t.Fatal("active clone Redo lost independent target state")
	}
}

func TestOpenTargetRefusalLeavesResultsAndStylesRetryable(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Editor.Insert("cat keep cat")
	first.Editor.ClearHistory()
	first.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(0, false)
	clickID(t, driver, newID)
	second := state.space.Current()
	second.Editor.Insert("cat stay cat")
	second.Editor.ClearHistory()
	second.Editor.Restrict(func(rune) bool { return true }, 0)
	state.query.Insert("cat")
	state.replacement.Insert("X")
	state.command("Find All in Open Documents")
	state.command("Replace All in Open Documents")
	if first.Editor.String() != "cat keep cat" || second.Editor.String() != "cat stay cat" || first.Editor.CanUndo() || second.Editor.CanUndo() ||
		!state.resultsOpen || !strings.Contains(state.notice, "no documents were changed") || !slices.Equal(state.styleSpans(first, 0), []text.Span{{Start: 4, End: 8}}) {
		t.Fatalf("later refusal lost source/results: %q / %q results=%v notice=%q", first.Editor.String(), second.Editor.String(), state.resultsOpen, state.notice)
	}
	second.Editor.Restrict(nil, 0)
	state.command("Replace All in Open Documents")
	if first.Editor.String() != "X keep X" || second.Editor.String() != "X stay X" || state.resultsOpen || !strings.HasPrefix(state.notice, "Unsaved:") {
		t.Fatalf("open target retry did not finish: %q / %q, %q", first.Editor.String(), second.Editor.String(), state.notice)
	}
}

func TestCaptureOpenTargetReplaceWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_OPEN_TARGET_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_OPEN_TARGET_SHOTS to capture cross-tab target replaces")
	}
	state := newModel()
	first := state.space.Current()
	first.Name = "first.txt"
	first.Editor.Insert("cat keep cat\nanchor")
	first.Editor.ClearHistory()
	first.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(0, false)
	first.GoToLine(2)
	state.toggleBookmark()
	second := state.space.NewDocument()
	state.showInActivePane(second)
	second.Name = "second.txt"
	second.Editor.Insert("cat stay cat\nanchor")
	second.Editor.ClearHistory()
	second.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(1, false)
	second.GoToLine(2)
	state.toggleBookmark()
	state.command("Split View")
	state.paneDocs[0], state.paneDocs[1] = first, second
	state.activatePane(1)
	state.query.Insert("cat")
	state.replacement.Insert("X")
	state.command("Replace All in Open Documents")
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
			t.Fatalf("%s open target layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "open-targets-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	capture("desktop", 1280, 800)
	capture("compact", 640, 400)
	state.activatePane(0)
	state.command("Undo")
	state.focusViewRequested, state.focusViewArmed = true, false
	if second.Editor.String() != "X stay X\nanchor" || first.Editor.String() != "cat keep cat\nanchor" {
		t.Fatal("screenshot Undo changed both sources")
	}
	capture("undo-first-compact", 640, 400)
}
