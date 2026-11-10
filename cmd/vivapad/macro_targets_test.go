package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"vivapad/internal/macro"
)

func TestMacroTargetsKeepFiveStylesFindMarkBookmarksAndCloneHistory(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	before, after := "alpha keep beta\nleft keep right", "// alpha keep beta\n// left keep right"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	for style := range 5 {
		doc.Editor.SetSelection(text.Range{Start: 6, End: 10})
		state.styleToken(style, true)
	}
	state.query.Insert("keep")
	state.command("Mark All")
	doc.GoToLine(2)
	state.toggleBookmark()
	state.command("Clone to Other View")
	doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 0})
	state.macroLast = []macro.Step{{Kind: macro.CommentStep, Comment: "line-add", LinePrefix: "//"}}
	state.command("Playback Macro")
	want := []text.Span{{Start: 9, End: 13}, {Start: 27, End: 31}}
	state.syncFindMarks(doc)
	if doc.Editor.String() != after || state.activePane != 1 || state.paneDocs[0] != doc || state.paneDocs[1] != doc || !slices.Equal(state.bookmarkLines(doc), []int{1}) || !slices.Equal(state.markedSpans, want) {
		t.Fatalf("macro changed clone/marks: %q, %q, find=%+v", doc.Editor.String(), state.notice, state.markedSpans)
	}
	for style := range 5 {
		if !slices.Equal(state.styleSpans(doc, style), want) {
			t.Fatalf("macro lost untouched style %d", style)
		}
	}
	selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
	clickID(t, driver, undoID)
	beforeSpans := []text.Span{{Start: 6, End: 10}, {Start: 21, End: 25}}
	state.syncFindMarks(doc)
	if doc.Editor.String() != before || doc.Editor.Caret() != 0 || doc.Editor.CanUndo() || !slices.Equal(state.styleSpans(doc, 0), beforeSpans) || !slices.Equal(state.markedSpans, beforeSpans) || !slices.Equal(state.bookmarkLines(doc), []int{1}) {
		t.Fatal("macro Undo lost untouched clone ranges or bookmark identity")
	}
	clickID(t, driver, redoID)
	state.syncFindMarks(doc)
	if doc.Editor.String() != after || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || !slices.Equal(state.styleSpans(doc, 4), want) || !slices.Equal(state.markedSpans, want) {
		t.Fatal("macro Redo lost final directional selection or untouched styles")
	}
}

func TestCaptureMacroTargetsWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_MACRO_TARGET_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_MACRO_TARGET_SHOTS to capture primitive macro commits")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Name = "macro-targets.go"
	doc.Editor.Insert("// alpha keep beta\nleft keep right")
	doc.Editor.ClearHistory()
	state.languages[doc] = ".go"
	state.configureCode(doc)
	doc.Editor.SetSelection(text.Range{Start: 9, End: 13})
	state.styleToken(0, true)
	doc.Editor.SetSelection(text.Range{Start: 14, End: 18})
	state.styleToken(1, false)
	state.query.Insert("alpha")
	state.command("Mark All")
	doc.GoToLine(2)
	state.toggleBookmark()
	state.command("Clone to Other View")
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	state.macroLast = []macro.Step{{Kind: macro.CommentStep, Comment: "line-toggle", LinePrefix: "//"}}
	state.command("Playback Macro")
	if doc.Editor.String() != "alpha keep beta\n// left keep right" || len(state.styleSpans(doc, 0)) != 2 || len(state.styleSpans(doc, 1)) != 1 {
		t.Fatal("macro screenshot did not retain untouched styles")
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
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
			t.Fatalf("%s macro layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "macro-targets-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	capture("desktop", 1280, 800)
	capture("compact", 640, 400)
	state.command("Undo")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	capture("undo-compact", 640, 400)
	state.command("Redo")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	capture("redo-compact", 640, 400)
}

func TestMacroEqualTargetDoesNotRestoreDeletedStyleSnapshot(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("aaaa")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 3, End: 4})
	state.styleToken(0, false)
	state.macroLast = []macro.Step{{Kind: macro.TextStep, TextKind: app.TextPaste, Text: "a"}}
	state.command("Playback Macro")
	if doc.Editor.String() != "aaaa" || len(state.styleSpans(doc, 0)) != 0 || !doc.Editor.CanUndo() {
		t.Fatal("equal-text playback skipped the actual styled target")
	}
	if !doc.Editor.Undo() || len(state.styleSpans(doc, 0)) != 0 || doc.Editor.CanUndo() {
		t.Fatal("equal-text macro Undo restored deleted styles or lost real history")
	}
}

func TestFailedMacroTargetsPreserveSourceIndicatorsAndBookmark(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("alpha keep\nanchor")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 6, End: 10})
	state.styleToken(0, false)
	doc.GoToLine(2)
	state.toggleBookmark()
	doc.Editor.MoveTo(0, false)
	steps := []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "!"}}
	if state.playMacroSteps(steps, 1, true) || !strings.Contains(state.notice, "advance") || doc.Editor.String() != "alpha keep\nanchor" || doc.Editor.Caret() != 0 || doc.Editor.CanUndo() || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 6, End: 10}}) || !slices.Equal(state.bookmarkLines(doc), []int{1}) {
		t.Fatal("failed EOF macro changed source indicators or bookmark")
	}
}
