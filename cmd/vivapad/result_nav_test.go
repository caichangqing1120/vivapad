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
	clickID(t, driver, state.resultRowID(1))
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

func TestResultShortcutPreservesCompositionOnRefusal(t *testing.T) {
	for _, tc := range []struct {
		name      string
		replace   bool
		modifiers gfx.Modifiers
		notice    string
	}{
		{"stale snapshot looks current", true, 0, staleSearchResultNotice},
		{"unchanged snapshot while composing", false, gfx.ModifierShift, composingResultNotice},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("cat cat")
			state.query.Insert("cat")
			state.findAllCurrent()
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, editorID)
			state.selectFileResult(0)
			committed, marked, displayed := "cat cat", "x", "xcat cat"
			if tc.replace {
				doc.Editor.ReplaceRange(text.Range{Start: 0, End: 3}, "")
				committed, marked, displayed = " cat", "cat", "cat cat"
			}
			doc.Editor.MoveTo(0, false)
			input := tentativeTextInput(t, state, 0)
			sendComposition(t, input, text.CompositionUpdate, marked)
			assertLiveComposition(t, doc)
			if doc.Editor.String() != displayed || doc.Editor.CommittedString() != committed {
				t.Fatal("fixture did not establish the expected visible and committed text")
			}
			selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
			if err := driver.Key(gfx.KeyF4, tc.modifiers); err != nil {
				t.Fatal(err)
			}
			if !doc.Editor.CompositionActive() || doc.Editor.String() != displayed || doc.Editor.CommittedString() != committed ||
				doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || state.resultSelected != 0 ||
				state.notice != tc.notice || driver.Snapshot().Focused != editorID {
				t.Fatalf("refused shortcut changed the edit or navigation: composing=%t text=%q selected=%d notice=%q",
					doc.Editor.CompositionActive(), doc.Editor.String(), state.resultSelected, state.notice)
			}
			sendComposition(t, input, text.CompositionCancel, "")
			if doc.Editor.String() != committed || tc.replace && (!doc.Editor.Undo() || doc.Editor.String() != "cat cat") {
				t.Fatal("refusal changed cancellation or undo")
			}
			if err := driver.Key(gfx.KeyF4, tc.modifiers); err != nil {
				t.Fatal(err)
			}
			if state.resultSelected != 1 || doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || state.notice != "" {
				t.Fatal("the same shortcut could not open the restored result")
			}
		})
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

func TestSearchResultMenuRefusalReturnsFocusToDraft(t *testing.T) {
	for _, command := range []string{"Next Search Result", "Previous Search Result"} {
		t.Run(command, func(t *testing.T) {
			state, driver := startDriver(t)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			doc.Editor.Insert("cat")
			state.query.Insert("cat")
			state.findAllCurrent()
			doc.Editor.Insert(" draft")
			const draft = "cat draft"
			doc.Editor.MoveTo(len(draft), false)
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(2))
			revealSearchMenuItem(t, driver, searchCommandIndex(t, command))
			clickID(t, driver, searchCommandID(t, command))
			if state.resultSelected != -1 || state.space.Current() != doc || doc.Editor.Caret() != len(draft) || state.notice != staleSearchResultNotice {
				t.Fatalf("refusal changed the draft or selected result: caret=%d result=%d notice=%q", doc.Editor.Caret(), state.resultSelected, state.notice)
			}
			requireEditorCommandFocus(t, driver, editorID)
			if err := driver.Type("x"); err != nil || doc.Editor.String() != draft+"x" {
				t.Fatalf("first key did not reach the original caret: %q, %v", doc.Editor.String(), err)
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
		func() error { clickID(t, driver, state.resultRowID(1)); return nil },
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
	state.replaceFileResults(result)
	state.resultsOpen = true
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
	clickID(t, driver, state.resultRowID(1))
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
		visible = visible || hit.ID == state.resultRowID(5)
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

func TestResultPaneResizePreservesSelectionAndManualScroll(t *testing.T) {
	for _, action := range []string{"shrink with selection visible", "shrink with focus on another row", "grow after scrolling away"} {
		t.Run(action, func(t *testing.T) {
			state, driver := startDriver(t)
			grow := action == "grow after scrolling away"
			otherFocus := action == "shrink with focus on another row"
			resize := func(width, height int) {
				t.Helper()
				if err := driver.Resize(width, height); err != nil {
					t.Fatal(err)
				}
			}
			row := func(id gfx.ID) (gfx.Rect, bool) {
				for _, hit := range driver.Snapshot().Hits {
					if hit.ID == id {
						return hit.Bounds, true
					}
				}
				return gfx.Rect{}, false
			}
			if !grow {
				resize(1000, 700)
			} else {
				resize(640, 400)
			}
			doc := state.space.Current()
			doc.Editor.Insert(strings.Repeat("cat\n", 20))
			state.query.Insert("cat")
			clickID(t, driver, menuScope.At(2))
			clickID(t, driver, searchCommandID(t, "Find All in Current Document"))
			index := state.resultVisible - 1
			if grow {
				index = 0
			}
			id := state.resultRowID(index)
			clickID(t, driver, id)
			beforeSelection := doc.Editor.Selection()
			wantFocus := id
			if otherFocus {
				for range index {
					if err := driver.Key(gfx.KeyTab, gfx.ModifierShift); err != nil {
						t.Fatal(err)
					}
				}
				if driver.Snapshot().Focused != state.resultRowID(0) || state.resultSelected != index {
					t.Fatal("Tab did not move focus independently of the selection")
				}
				wantFocus = resultsPaneID
			}
			if !grow {
				resize(640, 400)
				if _, ok := row(id); !ok || driver.Snapshot().Focused != wantFocus {
					t.Error("shrinking hid the selected result or lost its keyboard focus")
				}
			} else {
				bounds, ok := row(id)
				if !ok {
					t.Fatal("initial result row is missing")
				}
				for range 20 {
					if err := driver.Scroll(gfx.ScrollEvent{X: bounds.X + 10, Y: bounds.Y + 8, DeltaY: 120}); err != nil {
						t.Fatal(err)
					}
				}
				last := state.resultRowID(len(state.fileResults.Matches) - 1)
				if _, ok := row(last); !ok || state.resultOffset == 0 {
					t.Fatal("wheel did not scroll to the end of the results")
				}
				resize(1000, 700)
				if _, ok := row(id); ok {
					t.Error("growing jumped back to the selection after manual scrolling")
				}
				first := len(state.fileResults.Matches) - state.resultVisible
				if _, ok := row(state.resultRowID(first)); !ok {
					t.Error("growing left unused space instead of revealing more rows above the end")
				}
			}
			if state.resultSelected != index || doc.Editor.Selection() != beforeSelection {
				t.Fatal("resizing changed the selected result or the editor selection")
			}
			if otherFocus {
				if err := driver.Key(gfx.KeyUp, 0); err != nil {
					t.Fatal(err)
				}
				if state.resultSelected != index-1 || doc.Editor.Selection() != beforeSelection {
					t.Error("Up did not navigate within the result pane after resizing")
				}
			}
		})
	}
}

func TestResultNavigationOpensDirectoryMatchesAndRejectsMissingFile(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	firstPath, secondPath := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, driver := startDriver(t)
	state.replaceFileResults(searchfiles.Result{Matches: []searchfiles.Match{
		{Path: firstPath, Source: "cat", Line: 1, Column: 1, Text: "cat", Preview: "cat", Start: 0, End: 3},
		{Path: secondPath, Source: "cat", Line: 1, Column: 1, Text: "cat", Preview: "cat", Start: 0, End: 3},
	}})
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
