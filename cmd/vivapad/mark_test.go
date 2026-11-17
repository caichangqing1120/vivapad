package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestMarkAllPersistsAfterFindBarClosesAndCanClear(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("cat cat"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	clickID(t, driver, searchID)
	clickID(t, driver, queryID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Mark..."))
	if !state.markDialogOpen || len(state.markedSpans) != 0 {
		t.Fatal("Mark menu should open without marking")
	}
	clickID(t, driver, markAllID)
	if state.markedDocument != doc || len(state.markedSpans) != 2 {
		t.Fatalf("marked ranges = %+v doc=%p", state.markedSpans, state.markedDocument)
	}
	clickID(t, driver, markCloseID)
	if state.markDialogOpen || state.markedDocument != doc || len(state.markedSpans) != 2 {
		t.Fatalf("closing Mark cleared marks: open=%v marks=%+v", state.markDialogOpen, state.markedSpans)
	}
	state.query.SetSelection(text.Range{Start: 0, End: state.query.RuneCount()})
	state.query.Insert("dog")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(state.markedSpans) != 2 {
		t.Fatal("changing the query erased stored marks")
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Clear Marks"))
	if state.markedDocument != nil || len(state.markedSpans) != 0 {
		t.Fatal("Clear Marks left persistent highlights")
	}
}

func TestMarkAllInvalidRegexKeepsPreviousMarks(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	state.query.Insert("cat")
	state.command("Mark All")
	if len(state.markedSpans) != 2 {
		t.Fatal("initial marks missing")
	}
	state.query.SetSelection(text.Range{Start: 0, End: state.query.RuneCount()})
	state.query.Insert("(")
	state.searchOptions.Regex = true
	state.command("Mark All")
	if state.markedDocument != doc || len(state.markedSpans) != 2 || !strings.Contains(state.notice, "missing") {
		t.Fatalf("bad regex changed prior marks: %+v notice=%q", state.markedSpans, state.notice)
	}
}

func TestMarkedSpansFollowEditsAndClearOnSourceClose(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	state.query.Insert("cat")
	state.command("Mark All")
	if len(state.markedSpans) != 1 {
		t.Fatal("initial mark missing")
	}
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if state.markedDocument != state.space.Current() || len(state.markedSpans) != 1 || state.markedSpans[0] != (text.Span{Start: 0, End: 3}) {
		t.Fatalf("end-boundary insertion changed mark: %+v notice=%q", state.markedSpans, state.notice)
	}
	state.command("Mark All")
	if len(state.markedSpans) != 1 {
		t.Fatal("marking after edit failed")
	}
	clickID(t, driver, tabCloseScope.At(0))
	clickID(t, driver, discardID)
	if state.markedDocument != nil || len(state.markedSpans) != 0 {
		t.Fatal("closed source retained marks")
	}
}
