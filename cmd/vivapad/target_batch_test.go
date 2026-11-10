package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestReplaceAllMenuKeepsUntouchedIndicatorsAndSelectionHistory(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat keep cat")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(0, false)
	state.query.Insert("cat")
	state.replacement.Insert("X")
	clickID(t, driver, searchID)
	clickID(t, driver, replaceAllID)
	if doc.Editor.String() != "X keep X" || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 2, End: 6}}) || doc.Editor.Selection() != (text.Range{Start: 2, End: 6}) {
		t.Fatalf("Replace All affected untouched keep: %q spans=%+v selection=%v", doc.Editor.String(), state.styleSpans(doc, 0), doc.Editor.Selection())
	}
	clickID(t, driver, undoID)
	if doc.Editor.String() != "cat keep cat" || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 4, End: 8}}) || doc.Editor.Selection() != (text.Range{Start: 4, End: 8}) {
		t.Fatal("Replace All Undo did not restore untouched range/selection")
	}
	clickID(t, driver, redoID)
	if !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 2, End: 6}}) || doc.Editor.Selection() != (text.Range{Start: 2, End: 6}) {
		t.Fatal("Replace All Redo lost mapped selection")
	}
}

func TestAdjacentReplaceAllTargetsUseSourceForwardIndicatorOrder(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a12")
	doc.Editor.ClearHistory()
	for _, selection := range []text.Range{{Start: 0, End: 1}, {Start: 2, End: 3}} {
		doc.Editor.SetSelection(selection)
		state.styleToken(0, false)
	}
	if _, _, err := doc.ReplaceAllWithEdits("[12]", "X", document.SearchOptions{Regex: true}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "aXX" || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 0, End: 2}}) {
		t.Fatalf("target order did not follow source: %q %+v", doc.Editor.String(), state.styleSpans(doc, 0))
	}
	doc.Editor.Undo()
	if !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 0, End: 1}}) {
		t.Fatal("adjacent target Undo restored deleted style snapshots")
	}
	doc.Editor.Redo()
	if !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 0, End: 1}}) {
		t.Fatal("adjacent target Redo did not follow source mutation order")
	}
}

func TestEqualReplaceAllTargetsClearOnlyReplacedIndicatorRuns(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat keep cat")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(0, false)
	state.query.Insert("cat")
	state.command("Mark All")
	count, _, err := doc.ReplaceAllWithEdits("cat", "cat", document.SearchOptions{})
	state.syncFindMarks(doc)
	if err != nil || count != 2 || !doc.Editor.CanUndo() || len(state.markedSpans) != 0 || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 4, End: 8}}) {
		t.Fatalf("equal targets affected unrelated bytes: %v count=%d Find=%+v style=%+v", err, count, state.markedSpans, state.styleSpans(doc, 0))
	}
	if !doc.Editor.Undo() || doc.Editor.CanUndo() || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 4, End: 8}}) {
		t.Fatal("equal-target Undo lost real history or an untouched style")
	}
	state.syncFindMarks(doc)
	if len(state.markedSpans) != 0 {
		t.Fatal("equal-target Undo restored a deleted indicator snapshot")
	}
}

func TestColumnEditorMenuFollowsSourceEOLMergeAndReplacesContentStyles(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd\n")
	doc.Editor.ClearHistory()
	for _, selection := range []text.Range{{Start: 2, End: 3}, {Start: 5, End: 6}} {
		doc.Editor.SetSelection(selection)
		state.styleToken(0, false)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	state.styleToken(1, false)
	doc.Editor.MoveTo(0, false)
	state.command("Column Editor...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, columnTextID)
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, columnApplyID)
	if doc.Editor.String() != "Xab\nXcd\nX" || doc.Editor.Caret() != 0 || len(state.styleSpans(doc, 1)) != 0 || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 3, End: 8}}) {
		t.Fatalf("column content/EOL ranges = %q caret %d styles %+v / %+v", doc.Editor.String(), doc.Editor.Caret(), state.styleSpans(doc, 0), state.styleSpans(doc, 1))
	}
	state.command("Undo")
	if !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 2, End: 6}}) || len(state.styleSpans(doc, 1)) != 0 {
		t.Fatal("column Undo restored deleted content style or lost EOL indicators")
	}
}

func TestCaptureTargetBatchesWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_TARGET_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_TARGET_SHOTS to capture source target batches")
	}
	capture := func(state *model, name string, width, height int) {
		t.Helper()
		session, err := headless.Open(newConfig(state), nil, headless.Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s target batch layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "targets-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat keep cat\ncat keep cat")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(1, true)
	doc.Editor.MoveTo(0, false)
	if _, _, err := doc.ReplaceAllWithEdits("cat", "X", document.SearchOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(state.styleSpans(doc, 1)) != 2 {
		t.Fatal("Replace All screenshot lost untouched keep ranges")
	}
	capture(state, "replace-all-desktop", 1280, 800)
	capture(state, "replace-all-compact", 640, 400)
	state.command("Undo")
	capture(state, "replace-all-undo-compact", 640, 400)

	state = newModel()
	doc = state.space.Current()
	doc.Editor.Insert("ab\ncd\n")
	doc.Editor.ClearHistory()
	for _, selection := range []text.Range{{Start: 2, End: 3}, {Start: 5, End: 6}} {
		doc.Editor.SetSelection(selection)
		state.styleToken(0, false)
	}
	doc.Editor.MoveTo(0, false)
	if _, err := doc.EditColumn(document.ColumnEdit{Text: "X"}, 4); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 3, End: 8}}) {
		t.Fatal("Column Editor screenshot did not follow source gap merge")
	}
	capture(state, "column-editor-compact", 640, 400)
}
