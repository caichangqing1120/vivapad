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

func TestStyledIndicatorsFollowPinnedInsertionBoundaries(t *testing.T) {
	for _, test := range []struct {
		name     string
		position int
		inserted string
		want     text.Span
	}{
		{"before Unicode", 0, "猫", text.Span{Start: 6, End: 9}},
		{"at start", 3, "X", text.Span{Start: 4, End: 7}},
		{"inside", 4, "X", text.Span{Start: 3, End: 7}},
		{"at end", 6, "X", text.Span{Start: 3, End: 6}},
		{"document end", 9, "X", text.Span{Start: 3, End: 6}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert("aa cat bb")
			doc.Editor.SetSelection(text.Range{Start: 3, End: 6})
			state.styleToken(0, false)
			doc.Editor.MoveTo(test.position, false)
			doc.Editor.Insert(test.inserted)
			if got := state.styleSpans(doc, 0); !slices.Equal(got, []text.Span{test.want}) {
				t.Fatalf("insert at %d = %+v, want %+v", test.position, got, test.want)
			}
			if got := state.styleSpans(doc, 0); !slices.Equal(got, []text.Span{test.want}) {
				t.Fatal("second clone/view observation applied an edit twice")
			}
		})
	}
}

func TestIndicatorsUseActualRepeatedAndEqualTextReplacements(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("aaaa")
	doc.Editor.SetSelection(text.Range{Start: 3, End: 4})
	state.styleToken(0, false)
	doc.Editor.MoveTo(3, false)
	doc.Editor.Insert("a")
	if got := state.styleSpans(doc, 0); !slices.Equal(got, []text.Span{{Start: 4, End: 5}}) {
		t.Fatalf("repeated text used inferred rather than actual offset: %+v", got)
	}
	doc.Editor.SetSelection(text.Range{Start: 4, End: 5})
	doc.Editor.Insert("a")
	if got := state.styleSpans(doc, 0); len(got) != 0 || doc.Editor.String() != "aaaaa" {
		t.Fatalf("equal replacement did not delete then insert: %+v text=%q", got, doc.Editor.String())
	}
}

func TestIndicatorReplacementMergesDeletedGapBeforeInsertion(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab--cd")
	for _, selection := range []text.Range{{Start: 0, End: 2}, {Start: 4, End: 6}} {
		doc.Editor.SetSelection(selection)
		state.styleToken(0, false)
	}
	doc.Editor.SetSelection(text.Range{Start: 1, End: 4})
	doc.Editor.Insert("X")
	if got := state.styleSpans(doc, 0); !slices.Equal(got, []text.Span{{Start: 0, End: 4}}) || doc.Editor.String() != "aXcd" {
		t.Fatalf("merged replacement = %+v text=%q", got, doc.Editor.String())
	}
}

func TestIndicatorUndoReplaysMutationsInsteadOfRestoringStyleSnapshot(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("aa cat bb")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 3, End: 6})
	state.styleToken(0, false)
	doc.Editor.SetSelection(text.Range{Start: 4, End: 5})
	doc.Editor.Insert("")
	if got := state.styleSpans(doc, 0); !slices.Equal(got, []text.Span{{Start: 3, End: 5}}) {
		t.Fatalf("deleted interior = %+v", got)
	}
	if !doc.Editor.Undo() || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 3, End: 6}}) {
		t.Fatal("undo interior deletion failed to inherit the surviving indicator")
	}
	if !doc.Editor.Redo() || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 3, End: 5}}) {
		t.Fatal("redo interior deletion did not shrink indicator")
	}
	doc.Editor.SetSelection(text.Range{Start: 3, End: 5})
	doc.Editor.Insert("")
	if len(state.styleSpans(doc, 0)) != 0 || !doc.Editor.Undo() || len(state.styleSpans(doc, 0)) != 0 {
		t.Fatal("Undo incorrectly restored a separately deleted indicator snapshot")
	}
}

func TestFindMarkRebasesThroughNativeEditAndRejectsReplacedEditor(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	doc.Editor.ClearHistory()
	setMarkQuery(state, "cat")
	state.command("Mark All")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("é"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(state.markedSpans, []text.Span{{Start: 0, End: 5}, {Start: 6, End: 9}}) {
		t.Fatalf("Find Mark did not follow Unicode edit: %+v", state.markedSpans)
	}
	if !doc.Editor.Undo() {
		t.Fatal("source undo unavailable")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(state.markedSpans, []text.Span{{Start: 0, End: 3}, {Start: 4, End: 7}}) {
		t.Fatalf("Find Mark Undo = %+v", state.markedSpans)
	}
	doc.Editor = text.NewEditor("cat cat")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil || state.markedDocument != nil {
		t.Fatalf("replaced editor reused a foreign edit cursor: %v, %+v", err, state.markedSpans)
	}
}

func TestIndicatorJournalGapClearsRatherThanGuessingOffsets(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.styleToken(0, false)
	state.query.Insert("cat")
	state.command("Mark All")
	doc.Editor.MoveTo(1, false)
	for index := 0; index < text.EditJournalLimit+1; index++ {
		doc.Editor.Insert("x")
	}
	if len(state.styleSpans(doc, 0)) != 0 {
		t.Fatal("expired user-style cursor was applied partially")
	}
	state.syncFindMarks(doc)
	if state.markedDocument != nil || len(state.markedSpans) != 0 {
		t.Fatal("expired Find Mark cursor was applied partially")
	}
}

func TestEditedIndicatorsCopyTheirCurrentRuns(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.styleToken(0, true)
	state.query.Insert("cat")
	state.command("Mark All")
	doc.Editor.MoveTo(1, false)
	doc.Editor.Insert("!")
	state.copyStyles(0)
	if got := driver.Clipboard(); got != "c!at\r\ncat\r\n" {
		t.Fatalf("edited style copy = %q", got)
	}
	state.copyMarkedText()
	if got := driver.Clipboard(); got != "c!at\r\ncat\r\n" {
		t.Fatalf("edited Find Mark copy = %q", got)
	}
}

func TestIndicatorsKeepUntouchedGapsInDisjointBatchAndUndo(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("aaXbbYcc")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 3, End: 5})
	state.styleToken(0, false)
	state.query.Insert("bb")
	state.command("Mark All")
	if err := doc.Editor.ReplaceRanges([]text.Replacement{
		{Range: text.Range{Start: 2, End: 3}, Text: "猫"},
		{Range: text.Range{Start: 5, End: 6}, Text: "é"},
	}); err != nil {
		t.Fatal(err)
	}
	state.syncFindMarks(doc)
	want := []text.Span{{Start: 5, End: 7}}
	if !slices.Equal(state.styleSpans(doc, 0), want) || !slices.Equal(state.markedSpans, want) {
		t.Fatalf("untouched batch gap lost styles: %+v / %+v", state.styleSpans(doc, 0), state.markedSpans)
	}
	doc.Editor.Undo()
	state.syncFindMarks(doc)
	want = []text.Span{{Start: 3, End: 5}}
	if !slices.Equal(state.styleSpans(doc, 0), want) || !slices.Equal(state.markedSpans, want) {
		t.Fatalf("batch Undo changed untouched gap: %+v / %+v", state.styleSpans(doc, 0), state.markedSpans)
	}
}

func TestIndicatorsFollowMultiCaretAndColumnNativeInput(t *testing.T) {
	t.Run("multi caret", func(t *testing.T) {
		state, driver := startDriver(t)
		doc := state.space.Current()
		doc.Editor.Insert("cat keep cat")
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(text.Range{Start: 4, End: 8})
		state.styleToken(0, false)
		clickID(t, driver, editorID)
		doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
		state.multiSelectAll(document.SearchOptions{})
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		if err := driver.Type("X"); err != nil || doc.Editor.String() != "X keep X" || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 2, End: 6}}) {
			t.Fatalf("multi-caret indicator = %q %+v, %v", doc.Editor.String(), state.styleSpans(doc, 0), err)
		}
		state.command("Undo")
		if !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 4, End: 8}}) {
			t.Fatalf("multi-caret Undo = %+v", state.styleSpans(doc, 0))
		}
	})
	t.Run("column", func(t *testing.T) {
		state, driver := startDriver(t)
		doc := state.space.Current()
		doc.Editor.Insert("ab\ncd")
		doc.Editor.ClearHistory()
		for _, selection := range []text.Range{{Start: 1, End: 2}, {Start: 4, End: 5}} {
			doc.Editor.SetSelection(selection)
			state.styleToken(0, false)
		}
		clickID(t, driver, editorID)
		doc.Editor.MoveTo(1, false)
		state.command("Begin/End Select in Column Mode")
		doc.Editor.MoveTo(4, false)
		state.command("Begin/End Select in Column Mode")
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		if err := driver.Type("X"); err != nil || doc.Editor.String() != "aXb\ncXd" || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 2, End: 3}, {Start: 6, End: 7}}) {
			t.Fatalf("column indicator = %q %+v, %v", doc.Editor.String(), state.styleSpans(doc, 0), err)
		}
		state.command("Undo")
		if !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 1, End: 2}, {Start: 4, End: 5}}) {
			t.Fatalf("column Undo = %+v", state.styleSpans(doc, 0))
		}
	})
}

func TestIndicatorsFollowIMEPreeditCommitAndUndo(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.styleToken(0, true)
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Compose("ni"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Compose("你"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("你"); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "c你at cat" || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 0, End: 6}, {Start: 7, End: 10}}) {
		t.Fatalf("IME indicators = %q %+v", doc.Editor.String(), state.styleSpans(doc, 0))
	}
	state.command("Undo")
	if doc.Editor.String() != "cat cat" || !slices.Equal(state.styleSpans(doc, 0), []text.Span{{Start: 0, End: 3}, {Start: 4, End: 7}}) {
		t.Fatalf("IME Undo indicators = %q %+v", doc.Editor.String(), state.styleSpans(doc, 0))
	}
}

func TestCaptureIndicatorEditsWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_INDICATOR_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_INDICATOR_SHOTS to capture edited indicators")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat keep cat")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.styleToken(0, true)
	doc.Editor.SetSelection(text.Range{Start: 4, End: 8})
	state.styleToken(1, true)
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	capture := func(name string, first []text.Span) {
		t.Helper()
		if err := session.Resize(640, 400); err != nil {
			t.Fatal(err)
		}
		if got := state.styleSpans(doc, 0); !slices.Equal(got, first) {
			t.Fatalf("%s spans = %+v, want %+v", name, got, first)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s indicator layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "indicators-"+name+"-compact.png")); err != nil {
			t.Fatal(err)
		}
	}
	doc.Editor.MoveTo(1, false)
	doc.Editor.Insert("!")
	capture("edited", []text.Span{{Start: 0, End: 4}, {Start: 10, End: 13}})
	doc.Editor.Undo()
	capture("undo", []text.Span{{Start: 0, End: 3}, {Start: 9, End: 12}})
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	doc.Editor.Insert("")
	state.styleSpans(doc, 0)
	doc.Editor.Undo()
	doc.Editor.MoveTo(doc.Editor.Caret(), false)
	capture("undo-deleted-run", []text.Span{{Start: 9, End: 12}})
}
