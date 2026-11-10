package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func TestF7FocusesResultsAndEnterOpensSelectedMatch(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(7))
	clickID(t, driver, editorID)
	before := doc.Editor.Selection()
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != resultsPaneID || doc.Editor.Selection() != before {
		t.Fatalf("F7 did not focus results: focus=%v selection=%+v", driver.Snapshot().Focused, doc.Editor.Selection())
	}
	for _, key := range []ui.Key{ui.KeyDown, ui.KeyDown} {
		if err := driver.Key(key, 0); err != nil {
			t.Fatal(err)
		}
	}
	if state.resultSelected != 1 || doc.Editor.Selection() != before {
		t.Fatalf("result arrows changed editor: index=%d selection=%+v", state.resultSelected, doc.Editor.Selection())
	}
	if err := driver.Key(ui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || driver.Snapshot().Focused != editorID {
		t.Fatalf("Enter did not open selected match: selection=%+v focus=%v", doc.Editor.Selection(), driver.Snapshot().Focused)
	}
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != editorID || doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("F7 round trip lost editor position: focus=%v selection=%+v", driver.Snapshot().Focused, doc.Editor.Selection())
	}
}

func TestResultPaneKeyboardHomeEndPageAndF7FromClickedRow(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.space.Current().Editor.Insert(strings.Repeat("cat ", 20))
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(7))
	clickID(t, driver, editorID)
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyEnd, 0); err != nil {
		t.Fatal(err)
	}
	if state.resultSelected != 19 || state.resultOffset == 0 {
		t.Fatalf("End did not reveal final result: index=%d offset=%d", state.resultSelected, state.resultOffset)
	}
	if err := driver.Key(ui.KeyHome, 0); err != nil {
		t.Fatal(err)
	}
	if state.resultSelected != 0 || state.resultOffset != 0 {
		t.Fatalf("Home did not reveal first result: index=%d offset=%d", state.resultSelected, state.resultOffset)
	}
	if err := driver.Key(ui.KeyPageDown, 0); err != nil {
		t.Fatal(err)
	}
	if state.resultSelected < 2 {
		t.Fatalf("Page Down moved only to %d", state.resultSelected)
	}
	clickID(t, driver, resultScope.At(state.resultSelected))
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != editorID {
		t.Fatalf("F7 from clicked result kept focus at %v", driver.Snapshot().Focused)
	}
}

func TestF7WithoutResultsDoesNotStealFocus(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	before := driver.Snapshot().Focused
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != before || state.notice != "No search results" {
		t.Fatalf("F7 without results = focus %v notice %q", driver.Snapshot().Focused, state.notice)
	}
}

func TestFocusedResultPaneHasVisibleIndicatorBeforeRowSelection(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.space.Current().Editor.Insert("cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	expectedY := 400 - statusHeight - resultPaneHeight(400) + 1
	for _, primitive := range driver.Snapshot().Primitives {
		if primitive.Color == palette.blue && primitive.Bounds.Width >= 630 && primitive.Bounds.Height <= 3 && primitive.Bounds.Y >= expectedY-1 && primitive.Bounds.Y <= expectedY+1 {
			return
		}
	}
	t.Fatal("focused result pane has no visible focus indicator")
}

func TestSearchResultsWindowMenuFitsCompactViewportAndFocusesPane(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.space.Current().Editor.Insert("cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	bounds := revealSearchMenuItem(t, driver, 17)
	if bounds.Y+bounds.Height > 400-statusHeight {
		t.Fatalf("Search Results Window menu row clipped: %+v", bounds)
	}
	clickID(t, driver, menuItemScope.At(17))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != resultsPaneID {
		t.Fatalf("menu did not focus results: %v", driver.Snapshot().Focused)
	}
}

func TestF7ReturnsToActiveSplitEditor(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("cat left")
	right := state.space.NewDocument()
	right.Editor.Insert("cat right")
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(1)
	state.query.Insert("cat")
	state.findAllOpenDocuments()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != rightEditorID || state.space.Current() != right || state.activePane != 1 {
		t.Fatalf("F7 returned to wrong split pane: focused=%v active=%p pane=%d", driver.Snapshot().Focused, state.space.Current(), state.activePane)
	}
}

func TestStaleResultEnterKeepsResultPaneFocus(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	doc.Editor.Insert("dog")
	selection := doc.Editor.Selection()
	if err := driver.Key(ui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != resultsPaneID || doc.Editor.Selection() != selection || !strings.Contains(state.notice, "changed") {
		t.Fatalf("stale Enter left results: focus=%v selection=%+v notice=%q", driver.Snapshot().Focused, doc.Editor.Selection(), state.notice)
	}
}

func TestSearchResultsWindowMenuTogglesBackToEditor(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != resultsPaneID {
		t.Fatal("F7 did not establish result focus")
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(17))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != editorID {
		t.Fatalf("menu focus toggle returned to %v, want editor", driver.Snapshot().Focused)
	}
}

func TestNewSearchAfterClosingFocusedResultsCanFocusPaneAgain(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, resultsCloseID)
	state.findAllCurrent()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(17))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != resultsPaneID {
		t.Fatalf("new search inherited old result focus: %v", driver.Snapshot().Focused)
	}
}

func TestF7RevealsResultsHiddenByFindBarBeforeFocusing(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	state.searchOpen = true
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, queryID)
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	if state.searchOpen || driver.Snapshot().Focused != resultsPaneID {
		t.Fatalf("F7 focused hidden results: searchOpen=%v focus=%v", state.searchOpen, driver.Snapshot().Focused)
	}
}

func TestResultsMenuFromFindFieldDoesNotUseOldResultFocus(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("cat")
	state.query.Insert("cat")
	state.findAllCurrent()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Key(ui.KeyF7, 0); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchID)
	clickID(t, driver, queryID)
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(17))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.searchOpen || driver.Snapshot().Focused != resultsPaneID {
		t.Fatalf("menu used stale result focus: searchOpen=%v focus=%v", state.searchOpen, driver.Snapshot().Focused)
	}
}
