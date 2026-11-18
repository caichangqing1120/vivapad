package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/searchfiles"
)

func TestResultNavigationF4WrapsAcrossOpenDocumentsAndClickPosition(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Editor.Insert("cat one")
	second := state.space.NewDocument()
	second.Editor.Insert("cat two")
	state.showInActivePane(second)
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find All in Open Documents"))
	if state.resultSelected != -1 {
		t.Fatalf("new search selected result %d", state.resultSelected)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.space.Current() != first || state.resultSelected != 0 || first.Editor.Selection() != (text.Range{Start: 0, End: 3}) {
		t.Fatalf("first result navigation = active %p, selected %d", state.space.Current(), state.resultSelected)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.space.Current() != second || state.resultSelected != 1 {
		t.Fatalf("next result = active %p, selected %d", state.space.Current(), state.resultSelected)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.space.Current() != first || state.resultSelected != 0 {
		t.Fatalf("next result did not wrap: active %p, selected %d", state.space.Current(), state.resultSelected)
	}
	clickID(t, driver, resultScope.At(1))
	if state.resultSelected != 1 {
		t.Fatalf("click did not establish navigation position: %d", state.resultSelected)
	}
	if err := driver.Key(gfx.KeyF4, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if state.space.Current() != first || state.resultSelected != 0 {
		t.Fatalf("previous result did not navigate: active %p, selected %d", state.space.Current(), state.resultSelected)
	}
}

func TestF4AcrossSplitKeepsTheFirstEditInTheResultPane(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("cat cat")
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activatePane(0)
	state.query.Insert("cat")
	state.findAllOpenDocuments()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	left.Editor.MoveTo(4, false)
	right.Editor.MoveTo(0, false)
	right.Editor.Insert("cat ")
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != editorID || state.activePane != 0 || state.resultSelected != -1 || left.Editor.Caret() != 4 {
		t.Fatal("stale result changed focus, result or caret")
	}
	if !right.Editor.Undo() || right.Editor.String() != "cat cat" {
		t.Fatal("could not restore the exact result source")
	}
	if err := driver.Key(gfx.KeyF4, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if state.activePane != 1 || state.space.Current() != right || state.resultSelected != 1 || right.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatal("previous result did not select the right document and match")
	}
	if err := driver.Type("x"); err != nil || left.Editor.String() != "left" || right.Editor.String() != "cat x" {
		t.Fatalf("first edit after cross-pane F4: left=%q right=%q err=%v", left.Editor.String(), right.Editor.String(), err)
	}
	if !right.Editor.Undo() || right.Editor.String() != "cat cat" {
		t.Fatal("could not restore the result source after the first edit")
	}
	if err := driver.Key(gfx.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.resultSelected != 0 || driver.Snapshot().Focused != resultsPaneID {
		t.Fatal("F4 did not navigate while retaining search result list focus")
	}
}

func TestSearchResultMenuFocusesTheMatchedEditor(t *testing.T) {
	for _, command := range []string{"Next Search Result", "Previous Search Result"} {
		t.Run(command, func(t *testing.T) {
			state, driver := startDriver(t)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			left := state.space.Current()
			left.Editor.Insert("left")
			right := state.space.NewDocument()
			right.Editor.Insert("cat")
			state.toggleSplit()
			state.paneDocs = [2]*document.Document{left, right}
			state.paneOwner[left], state.paneOwner[right] = 0, 1
			state.activatePane(0)
			state.query.Insert("cat")
			state.findAllOpenDocuments()
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, editorID)
			clickID(t, driver, menuScope.At(2))
			revealSearchMenuItem(t, driver, searchCommandIndex(t, command))
			clickID(t, driver, searchCommandID(t, command))
			if state.activePane != 1 || state.space.Current() != right || right.Editor.Selection() != (text.Range{Start: 0, End: 3}) {
				t.Fatal("menu did not select the match in the right pane")
			}
			if err := driver.Type("x"); err != nil || left.Editor.String() != "left" || right.Editor.String() != "x" {
				t.Fatalf("first edit after menu navigation: left=%q right=%q, %v", left.Editor.String(), right.Editor.String(), err)
			}
		})
	}
}

func TestStaleResultNavigationLeavesSelectionAndIndexUntouched(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find All in Current Document"))
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	doc.Editor.Insert("dog")
	selection := doc.Editor.Selection()
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.resultSelected != -1 || doc.Editor.Selection() != selection || !strings.Contains(state.notice, "changed") {
		t.Fatalf("stale result changed navigation: selected=%d selection=%+v notice=%q", state.resultSelected, doc.Editor.Selection(), state.notice)
	}
}

func TestRepeatedUnicodeResultRejectsStaleKeyboardAndPointerNavigation(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("猫 cat 猫 cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(0, false)
	doc.Editor.Insert("猫 cat ")
	before, value := doc.Editor.Selection(), doc.Editor.String()
	for _, activate := range []func() error{
		func() error { return driver.Key(gfx.KeyF4, 0) },
		func() error { clickID(t, driver, resultScope.At(1)); return nil },
	} {
		if err := activate(); err != nil {
			t.Fatal(err)
		}
		if state.resultSelected != -1 || doc.Editor.Selection() != before || state.space.Current() != doc ||
			doc.Editor.String() != value || !strings.Contains(state.notice, "changed") {
			t.Fatalf("stale activation changed state: selected=%d selection=%+v notice=%q", state.resultSelected, doc.Editor.Selection(), state.notice)
		}
	}
	if !doc.Editor.Undo() || !state.openFileResult(1) || state.resultSelected != 1 || doc.Editor.Selection() != (text.Range{Start: 8, End: 11}) {
		t.Fatalf("restored source rejected: selected=%d selection=%+v notice=%q", state.resultSelected, doc.Editor.Selection(), state.notice)
	}
}

func TestDirectoryResultRefusesChangedFileBeforeOpeningTab(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "result.txt")
	if err := os.WriteFile(path, []byte("cat cat"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := searchfiles.Find(t.Context(), root, "cat", document.SearchOptions{}, searchfiles.Options{})
	if err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	state.fileResults, state.resultsOpen = result, true
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	current := state.space.Current()
	current.Editor.Insert("keep")
	selection := current.Editor.Selection()
	if err := os.WriteFile(path, []byte("cat cat cat"), 0600); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, resultScope.At(1))
	if state.resultSelected != -1 || state.space.Current() != current || len(state.space.Documents) != 1 ||
		current.Editor.Selection() != selection || current.Editor.String() != "keep" || !strings.Contains(state.notice, "changed") {
		t.Fatalf("changed directory result opened a tab: selected=%d docs=%d notice=%q", state.resultSelected, len(state.space.Documents), state.notice)
	}
	if err := os.WriteFile(path, []byte("cat cat"), 0600); err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if !state.openFileResult(1) || state.resultSelected != 1 || len(state.space.Documents) != 2 || state.space.Current().Path != canonical ||
		state.notice == "Search result changed; run the search again" {
		t.Fatalf("restored directory source rejected: selected=%d docs=%d notice=%q", state.resultSelected, len(state.space.Documents), state.notice)
	}
}

func TestResultNavigationAdjustsAfterClosingAResultSource(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles(paths)
	second := state.space.Documents[1]
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find All in Open Documents"))
	for range 2 {
		if err := driver.Key(gfx.KeyF4, 0); err != nil {
			t.Fatal(err)
		}
	}
	state.closeTab(0)
	if state.resultSelected != 0 || len(state.fileResults.Matches) != 1 {
		t.Fatalf("closing earlier source left stale index %d / %d", state.resultSelected, len(state.fileResults.Matches))
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.resultSelected != 0 || state.space.Current() != second {
		t.Fatalf("remaining result was not navigable: %d, %p", state.resultSelected, state.space.Current())
	}
	state.closeTab(state.documentIndex(second))
	if state.resultsOpen || state.resultSelected != -1 || len(state.fileResults.Matches) != 0 {
		t.Fatalf("last result source close left navigation state: open=%v index=%d matches=%d", state.resultsOpen, state.resultSelected, len(state.fileResults.Matches))
	}
}

func TestResultNavigationKeepsSelectedRowVisibleAndMenuReachable(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.space.Current().Editor.Insert(strings.Repeat("cat ", 20))
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find All in Current Document"))
	for range 6 {
		if err := driver.Key(gfx.KeyF4, 0); err != nil {
			t.Fatal(err)
		}
	}
	if state.resultSelected != 5 || state.resultOffset == 0 {
		t.Fatalf("selected row was not scrolled into view: selected=%d offset=%d", state.resultSelected, state.resultOffset)
	}
	visible := false
	for _, hit := range driver.Snapshot().Hits {
		visible = visible || hit.ID == resultScope.At(5)
	}
	if !visible {
		t.Fatal("selected result row is not in the visible hit list")
	}
	clickID(t, driver, menuScope.At(2))
	for _, command := range []string{"Next Search Result", "Previous Search Result"} {
		index := searchCommandIndex(t, command)
		bounds := revealSearchMenuItem(t, driver, index)
		if bounds.Y+bounds.Height > 400-statusHeight {
			t.Fatalf("result navigation menu item %d clipped: %+v", index, bounds)
		}
	}
	clickID(t, driver, searchCommandID(t, "Previous Search Result"))
	if state.resultSelected != 4 {
		t.Fatalf("previous-result menu selected %d, want 4", state.resultSelected)
	}
}

func TestResultNavigationOpensDirectoryMatchesAndRejectsMissingFile(t *testing.T) {
	root := t.TempDir()
	firstPath, secondPath := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, driver := startDriver(t)
	state.fileResults = searchfiles.Result{Matches: []searchfiles.Match{
		{Path: firstPath, Source: "cat", Line: 1, Column: 1, Text: "cat", Preview: "cat", Start: 0, End: 3},
		{Path: secondPath, Source: "cat", Line: 1, Column: 1, Text: "cat", Preview: "cat", Start: 0, End: 3},
	}}
	state.resultsOpen, state.resultSelected, state.resultQuery = true, -1, "cat"
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.space.Current().Path == "" || state.space.Current().Name != "first.txt" || state.resultSelected != 0 {
		t.Fatalf("first disk result = %+v, index=%d", state.space.Current(), state.resultSelected)
	}
	if err := os.Remove(secondPath); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.resultSelected != 0 || state.space.Current().Name != "first.txt" || !strings.Contains(state.notice, "second.txt") {
		t.Fatalf("missing disk result changed selection: index=%d name=%q notice=%q", state.resultSelected, state.space.Current().Name, state.notice)
	}
}

func TestResultNavigationWithoutMatchesDoesNotMoveCaret(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("no match")
	before := doc.Editor.Selection()
	state.resultsOpen = true
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Selection() != before || state.resultSelected != -1 || state.notice != "No search results" {
		t.Fatalf("empty results moved editor: selection=%+v index=%d notice=%q", doc.Editor.Selection(), state.resultSelected, state.notice)
	}
}

func TestFailedNewSearchResetsPreviousResultPosition(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.resultSelected != 0 {
		t.Fatalf("initial result index = %d", state.resultSelected)
	}
	state.query.SetSelection(text.Range{Start: 0, End: state.query.RuneCount()})
	state.query.Insert("(")
	state.searchOptions.Regex = true
	state.findAllCurrent()
	if state.resultsOpen || state.resultSelected != -1 {
		t.Fatalf("failed search retained result navigation: open=%v index=%d", state.resultsOpen, state.resultSelected)
	}
}
