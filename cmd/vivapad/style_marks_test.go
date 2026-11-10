package main

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestFiveStylesAccumulateWithSourceDefaultsWithoutEditing(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat CAT scatter cat")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	before, dirty := doc.Editor.Selection(), doc.Dirty()
	state.searchOptions.Regex, state.searchOptions.MatchCase = true, true
	state.styleToken(0, true)
	want := []text.Span{{Start: 0, End: 3}, {Start: 4, End: 7}, {Start: 16, End: 19}}
	if got := state.styleSpans(doc, 0); !slices.Equal(got, want) {
		t.Fatalf("default style-all matches = %+v, want %+v", got, want)
	}
	doc.Editor.SetSelection(text.Range{Start: 15, End: 8})
	state.styleToken(1, false)
	if got := state.styleSpans(doc, 1); !slices.Equal(got, []text.Span{{Start: 8, End: 15}}) || doc.Editor.Caret() != 8 {
		t.Fatalf("single style selection = %+v caret %d", got, doc.Editor.Caret())
	}
	doc.Editor.MoveTo(1, false)
	state.styleToken(1, false)
	if len(state.styleSpans(doc, 1)) != 2 || !slices.Equal(state.styleSpans(doc, 0), want) {
		t.Fatalf("styles did not independently accumulate: %+v / %+v", state.styleSpans(doc, 0), state.styleSpans(doc, 1))
	}
	if doc.Editor.String() != "cat CAT scatter cat" || doc.Editor.CanUndo() || doc.Dirty() != dirty || doc.Editor.Selection() != before {
		t.Fatalf("styling changed document state: text=%q selection=%v", doc.Editor.String(), doc.Editor.Selection())
	}
}

func TestStyleCopyClearAndNavigationDoNotAffectFindMarks(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("pre cat then cat")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 4, End: 7})
	state.styleToken(0, true)
	state.styleToken(1, false)
	state.command("Mark...")
	setMarkQuery(state, "cat")
	state.markAll()
	state.copyStyles(-1)
	if got := driver.Clipboard(); got != "cat\r\ncat\r\ncat\r\n" {
		t.Fatalf("copy all styles did not retain overlapping style runs: %q", got)
	}
	state.clearStyles(0)
	if len(state.styleSpans(doc, 0)) != 0 || len(state.styleSpans(doc, 1)) != 1 || len(state.markedSpans) != 2 {
		t.Fatal("clearing one style affected another indicator")
	}
	doc.Editor.MoveTo(0, false)
	state.navigateStyle(1, false)
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("style navigation = %v", doc.Editor.Selection())
	}
	state.clearStyles(-1)
	if len(state.styleSpans(doc, 1)) != 0 || len(state.markedSpans) != 2 || doc.Editor.CanUndo() {
		t.Fatal("Clear all Styles changed Find Mark or Undo")
	}
	driver.SetClipboard("unchanged")
	state.copyStyles(1)
	if driver.Clipboard() != "unchanged" {
		t.Fatal("empty style replaced clipboard")
	}
}

func TestStylesRejectStaleRangesAndShareCloneState(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	doc.Editor.MoveTo(1, false)
	state.styleToken(4, true)
	state.command("Clone to Other View")
	state.activatePane(1)
	if len(state.styleSpans(state.paneDocument(1), 4)) != 2 {
		t.Fatal("clone did not share indicator state")
	}
	doc.Editor = text.NewEditor(doc.Editor.String())
	selection := doc.Editor.Selection()
	driver.SetClipboard("prior")
	state.copyStyles(4)
	state.navigateStyle(4, false)
	if len(state.styleSpans(doc, 4)) != 0 || driver.Clipboard() != "prior" || doc.Editor.Selection() != selection {
		t.Fatal("stale indicator copied or navigated unrelated bytes")
	}
}
