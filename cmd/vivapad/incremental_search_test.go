package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func TestIncrementalSearchSeedsSelectionAndKeepsOrdinaryFindState(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat catalog")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.query = text.NewEditor("ordinary")
	options := document.SearchOptions{MatchCase: true, WholeWord: true, Regex: true}
	state.searchOptions = options
	state.command("Incremental Search")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.incrementOpen || state.incrementQuery.String() != "cat" || !state.incrementCount ||
		state.query.String() != "ordinary" || state.searchOptions != options || driver.Snapshot().Focused != incrementQueryID || state.incrementStatus != "1/3" {
		t.Fatalf("incremental start = open:%v query:%q status:%q focus:%v", state.incrementOpen, state.incrementQuery.String(), state.incrementStatus, driver.Snapshot().Focused)
	}
	clickID(t, driver, incrementNextID)
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || state.incrementStatus != "2/3" {
		t.Fatalf("incremental Next = %+v status=%q", doc.Editor.Selection(), state.incrementStatus)
	}
	clickID(t, driver, incrementPreviousID)
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) || state.incrementStatus != "1/3" {
		t.Fatalf("incremental Previous = %+v status=%q", doc.Editor.Selection(), state.incrementStatus)
	}
}

func TestIncrementalSearchNativeTypingFiltersCountAndHighlights(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("alpha alphabet alpha")
	doc.Editor.MoveTo(0, false)
	state.query = text.NewEditor("leave unchanged")
	state.command("Incremental Search")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("alp"); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) || state.incrementStatus != "1/3" {
		t.Fatalf("typed incremental = %+v status=%q", doc.Editor.Selection(), state.incrementStatus)
	}
	clickID(t, driver, incrementHighlightID)
	if !state.incrementHighlight || len(state.incrementMatches) != 3 {
		t.Fatalf("highlight-all = %v, matches=%d", state.incrementHighlight, len(state.incrementMatches))
	}
	clickID(t, driver, incrementQueryID)
	if err := driver.Type("hab"); err != nil {
		t.Fatal(err)
	}
	if state.incrementQuery.String() != "alphab" || doc.Editor.Selection() != (text.Range{Start: 6, End: 12}) || state.incrementStatus != "1/1" || state.query.String() != "leave unchanged" {
		t.Fatalf("refined incremental = %q %+v %q", state.incrementQuery.String(), doc.Editor.Selection(), state.incrementStatus)
	}
	if err := driver.Key(ui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "alpha alphabet alpha" || !strings.Contains(state.incrementStatus, "Reached end") {
		t.Fatalf("incremental Enter inserted text or missed wrap: %q status=%q", doc.Editor.String(), state.incrementStatus)
	}
}

func TestIncrementalSearchCaseCountNoMatchAndEscape(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("Cat cat CAT")
	doc.Editor.MoveTo(0, false)
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(0, false)
	if handled, err := driver.ShortcutChord(primary()|ui.ModifierAlt, 'i'); err != nil || !handled {
		t.Fatalf("incremental shortcut handled=%v err=%v", handled, err)
	}
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	if state.incrementStatus != "1/3" {
		t.Fatalf("initial count = %q", state.incrementStatus)
	}
	clickID(t, driver, incrementMatchCaseID)
	if !state.incrementMatchCase || doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || state.incrementStatus != "1/1" {
		t.Fatalf("case toggle = %+v status=%q", doc.Editor.Selection(), state.incrementStatus)
	}
	clickID(t, driver, incrementCountID)
	if state.incrementCount || strings.Contains(state.incrementStatus, "/") {
		t.Fatalf("count toggle = %v status=%q", state.incrementCount, state.incrementStatus)
	}
	clickID(t, driver, incrementQueryID)
	if err := driver.SelectText(0, 3); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("missing"); err != nil {
		t.Fatal(err)
	}
	if state.incrementStatus != "Phrase not found" || doc.Editor.String() != "Cat cat CAT" {
		t.Fatalf("not found = %q text=%q", state.incrementStatus, doc.Editor.String())
	}
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.incrementOpen || len(state.incrementMatches) != 0 || driver.Snapshot().Focused != editorID || state.query.String() != "" {
		t.Fatalf("incremental Escape = open:%v focus:%v marks:%d", state.incrementOpen, driver.Snapshot().Focused, len(state.incrementMatches))
	}
}

func TestIncrementalSearchMenuFitsCompactAndReadonlySplitPane(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	left := state.space.Current()
	left.Editor.Insert("left left")
	right := state.space.NewDocument()
	right.Editor.Insert("right right")
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	state.readOnly[right] = true
	right.Editor.MoveTo(0, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	bounds := revealSearchMenuItem(t, driver, 19)
	if bounds.Y+bounds.Height > 400-statusHeight {
		t.Fatalf("incremental menu row clipped: %+v", bounds)
	}
	clickID(t, driver, menuItemScope.At(19))
	if !state.incrementOpen || driver.Snapshot().Focused != incrementQueryID {
		t.Fatal("menu did not focus incremental input")
	}
	if err := driver.Type("right"); err != nil {
		t.Fatal(err)
	}
	if right.Editor.Selection() != (text.Range{Start: 0, End: 5}) || left.Editor.String() != "left left" || right.Editor.String() != "right right" {
		t.Fatal("read-only split incremental search changed text or pane")
	}
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != rightEditorID {
		t.Fatalf("Escape did not return to right editor: %v", driver.Snapshot().Focused)
	}
}

func TestIncrementalSearchCountsOverlappingResults(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("banana")
	doc.Editor.MoveTo(0, false)
	state.command("Incremental Search")
	state.incrementQuery.Insert("ana")
	state.findIncremental(false, false)
	if doc.Editor.Selection() != (text.Range{Start: 1, End: 4}) || state.incrementStatus != "1/2" {
		t.Fatalf("first overlap count = %+v status=%q", doc.Editor.Selection(), state.incrementStatus)
	}
	state.findIncremental(false, true)
	if doc.Editor.Selection() != (text.Range{Start: 3, End: 6}) || state.incrementStatus != "2/2" {
		t.Fatalf("next overlap count = %+v status=%q", doc.Editor.Selection(), state.incrementStatus)
	}
}

func TestIncrementalCaseMissCollapsesSelectionAndKeepsText(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("Cat CAT")
	doc.Editor.MoveTo(0, false)
	state.command("Incremental Search")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) {
		t.Fatal("insensitive match missing")
	}
	clickID(t, driver, incrementMatchCaseID)
	if state.incrementStatus != "Phrase not found" || doc.Editor.Selection() != (text.Range{Start: 0, End: 0}) || doc.Editor.String() != "Cat CAT" {
		t.Fatalf("case miss = status:%q selection:%+v text:%q", state.incrementStatus, doc.Editor.Selection(), doc.Editor.String())
	}
}

func TestIncrementalIMEWaitsForCommitAndResultsReturnOnClose(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("猫 猫")
	doc.Editor.MoveTo(0, false)
	state.query = text.NewEditor("猫")
	state.command("Find All in Current Document")
	if !state.resultsOpen {
		t.Fatal("results pane did not open")
	}
	state.command("Incremental Search")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == resultsPaneID {
			t.Fatal("results pane overlapped incremental band")
		}
	}
	if err := driver.Compose("m"); err != nil {
		t.Fatal(err)
	}
	if state.incrementStatus != "" || doc.Editor.Selection() != (text.Range{Start: 0, End: 0}) {
		t.Fatalf("uncommitted IME searched: status=%q selection=%+v", state.incrementStatus, doc.Editor.Selection())
	}
	if err := driver.Type("猫"); err != nil {
		t.Fatal(err)
	}
	if state.incrementQuery.String() != "猫" || state.incrementStatus != "1/2" || doc.Editor.Selection() != (text.Range{Start: 0, End: 1}) {
		t.Fatalf("committed IME = query:%q status:%q selection:%+v", state.incrementQuery.String(), state.incrementStatus, doc.Editor.Selection())
	}
	clickID(t, driver, incrementCloseID)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.resultsOpen || state.incrementOpen || driver.Snapshot().Focused != editorID {
		t.Fatalf("closing incremental lost results: results=%v focused=%v", state.resultsOpen, driver.Snapshot().Focused)
	}
}

func TestIncrementalFocusedF3AndRepeatShortcutUseSeparateQuery(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat cat")
	doc.Editor.MoveTo(0, false)
	state.query = text.NewEditor("ordinary")
	state.command("Incremental Search")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyF3, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("focused incremental F3 = %+v", doc.Editor.Selection())
	}
	if err := driver.Key(ui.KeyF3, ui.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) {
		t.Fatalf("focused incremental Shift+F3 = %+v", doc.Editor.Selection())
	}
	if handled, err := driver.ShortcutChord(primary()|ui.ModifierAlt, 'i'); err != nil || !handled {
		t.Fatalf("repeat incremental shortcut handled=%v err=%v", handled, err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || state.query.String() != "ordinary" || state.incrementQuery.String() != "cat" {
		t.Fatalf("repeat shortcut = %+v ordinary=%q increment=%q", doc.Editor.Selection(), state.query.String(), state.incrementQuery.String())
	}
}
