package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestFindAllCurrentUnsavedDocumentNavigatesWithoutOpeningFile(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("猫 cat\ncat cat"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	clickID(t, driver, searchID)
	clickID(t, driver, queryID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, findAllCurrentID)
	if !state.resultsOpen || state.searchOpen || state.resultDocument != doc || len(state.fileResults.Matches) != 3 {
		t.Fatalf("current results = %+v open=%v source=%p", state.fileResults, state.resultsOpen, state.resultDocument)
	}
	clickID(t, driver, resultScope.At(1))
	selected := doc.Editor.Selection()
	line, column := doc.Position()
	if state.space.Current() != doc || doc.Path != "" || string([]rune(doc.Editor.String())[selected.Start:selected.End]) != "cat" ||
		state.fileResults.Matches[1].Line != 2 || state.fileResults.Matches[1].Column != 1 || line != 2 || column != 4 {
		t.Fatalf("clicked in-memory result = %+v selection=%+v position=%d:%d", doc, selected, line, column)
	}
	if len(state.space.Documents) != 1 || !doc.Dirty() {
		t.Fatal("find-all changed the unsaved document lifecycle")
	}
}

func TestFindAllCurrentRejectsStaleResultAndInvalidPattern(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("one one"); err != nil {
		t.Fatal(err)
	}
	state.query.Insert("one")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find All in Current Document"))
	if len(state.fileResults.Matches) != 2 {
		t.Fatalf("initial results = %+v", state.fileResults)
	}
	doc := state.space.Current()
	doc.Editor.MoveTo(0, false)
	doc.Editor.Insert("prefix ")
	clickID(t, driver, resultScope.At(1))
	if !strings.Contains(state.notice, "changed") || state.space.Current() != doc || len(state.space.Documents) != 1 {
		t.Fatalf("stale result selection = notice:%q doc:%+v", state.notice, state.space.Current())
	}
	state.resultsOpen = false
	state.searchOptions.Regex = true
	state.query.SetSelection(text.Range{Start: 0, End: state.query.RuneCount()})
	state.query.Insert("(")
	state.command("Find All in Current Document")
	if state.resultsOpen || !strings.Contains(state.notice, "missing") {
		t.Fatalf("invalid regex exposed results: open=%v notice=%q", state.resultsOpen, state.notice)
	}
}

func TestSearchResultRefusesPreeditThatResemblesItsSnapshot(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	if !state.openFileResult(0) {
		t.Fatal("precondition: could not navigate the original result")
	}
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: 3}, "dog")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	doc.Editor.SetMarkedText("cat", text.Range{Start: 2, End: 2})
	if doc.Editor.String() != "cat cat" || doc.Editor.CommittedString() != "dog cat" {
		t.Fatal("precondition: preedit does not resemble the old source")
	}
	selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
	if state.openFileResult(1) || state.resultSelected != 0 || state.space.Current() != doc ||
		!doc.Editor.CompositionActive() || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret ||
		doc.Editor.CommittedString() != "dog cat" || state.notice != staleSearchResultNotice {
		t.Fatalf("stale result changed the composing source: selected=%d, text=%q, committed=%q, notice=%q",
			state.resultSelected, doc.Editor.String(), doc.Editor.CommittedString(), state.notice)
	}
	if !doc.Editor.CancelComposition() || !doc.Editor.Undo() || doc.Editor.String() != "cat cat" {
		t.Fatal("refused navigation changed the cancel or Undo path")
	}
	doc.Editor.MoveTo(0, false)
	doc.Editor.SetMarkedText("x", text.Range{})
	if state.openFileResult(1) || state.notice != composingResultNotice || state.resultSelected != 0 ||
		!doc.Editor.CompositionActive() || doc.Editor.CommittedString() != "cat cat" {
		t.Fatalf("unchanged source did not keep its composition: %q", state.notice)
	}
	if !doc.Editor.CancelComposition() || !state.openFileResult(1) || state.resultSelected != 1 ||
		doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || state.notice != "" {
		t.Fatalf("restored source could not reuse the old result: %q", state.notice)
	}
}

func TestFindAllCurrentCapsResultsAndReleasesClosedDocument(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert(strings.Repeat("x ", 2001))
	state.query.Insert("x")
	state.command("Find All in Current Document")
	if !state.fileResults.Truncated || len(state.fileResults.Matches) != 2000 || state.resultDocument != doc {
		t.Fatalf("capped results = %+v source=%p", state.fileResults, state.resultDocument)
	}
	clickID(t, driver, tabCloseScope.At(state.tabHitIndex(state.space.Documents[0])))
	clickID(t, driver, discardID)
	if state.resultsOpen || state.resultDocument != nil {
		t.Fatal("closed document left active search results")
	}
	if state.fileResults.Matches != nil || state.resultQuery != "" {
		t.Fatal("closing the source retained its search snapshot")
	}
}

func TestDismissFinderReleasesSearchSnapshot(t *testing.T) {
	for _, closeWith := range []string{"close button", "Escape"} {
		t.Run(closeWith, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("cat cat")
			state.query.Insert("cat")
			state.findAllOpenDocuments()
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, resultScope.At(0))
			before := doc.Editor.Selection()
			if state.resultSelected != 0 || len(state.fileResults.Matches) != 2 || len(state.resultDocuments) != 2 {
				t.Fatal("fixture did not select a source-backed result")
			}
			if closeWith == "close button" {
				clickID(t, driver, resultsCloseID)
			} else if err := driver.Key(gfx.KeyEscape, 0); err != nil {
				t.Fatal(err)
			}
			if state.resultsOpen || state.resultSelected != -1 || state.fileResults.Matches != nil ||
				state.resultDocuments != nil || state.resultQuery != "" || state.resultOpenDocuments {
				t.Fatal("dismissed Finder retained the source snapshot or navigation state")
			}
			if err := driver.Key(gfx.KeyF4, 0); err != nil {
				t.Fatal(err)
			}
			if doc.Editor.Selection() != before || doc.Editor.String() != "cat cat" || state.space.Current() != doc {
				t.Fatal("dismissing Finder or navigating afterward changed the editor")
			}
		})
	}
}

func TestFindAllResultHeadingKeepsTheQueryThatProducedIt(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("cat dog"); err != nil {
		t.Fatal(err)
	}
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find All in Current Document"))
	state.query.SetSelection(text.Range{Start: 0, End: state.query.RuneCount()})
	state.query.Insert("dog")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, "Find result: cat") {
			found = true
		}
	}
	if !found {
		t.Fatal("result heading changed after the search query was edited")
	}
}

func TestCurrentResultDoesNotMoveTabToOtherView(t *testing.T) {
	root := t.TempDir()
	firstPath, secondPath := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	if err := os.WriteFile(firstPath, []byte("cat"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{firstPath, secondPath})
	first, second := state.space.Documents[0], state.space.Documents[1]
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{first, second}
	state.paneOwner[first], state.paneOwner[second] = 0, 1
	state.activePane = 0
	state.space.Select(0)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find All in Current Document"))
	clickID(t, driver, rightEditorID)
	clickID(t, driver, resultScope.At(0))
	if state.activePane != 0 || state.paneDocs[0] != first || state.paneDocs[1] != second || state.paneOwner[first] != 0 || state.paneOwner[second] != 1 {
		t.Fatalf("current-result navigation moved views: active=%d panes=%+v owners=%+v", state.activePane, state.paneDocs, state.paneOwner)
	}
}

func TestDirectoryResultHeadingKeepsSubmittedQuery(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.txt"), []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	state.filesRoot = text.NewEditor(root)
	state.query.Insert("one")
	state.startFileSearch()
	state.query.SetSelection(text.Range{Start: 0, End: state.query.RuneCount()})
	state.query.Insert("two")
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.filesSearching || state.resultQuery != "one" || len(state.fileResults.Matches) != 1 {
		t.Fatalf("directory query snapshot = %q result=%+v", state.resultQuery, state.fileResults)
	}
	found := false
	for _, label := range driver.Snapshot().Labels {
		found = found || strings.Contains(label.Value, "Find result: one")
	}
	if !found {
		t.Fatal("directory result heading followed a later query edit")
	}
}
