package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestSelectAndFindUsesLiteralSelectionButKeepsStoredOptions(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a.b axb a.b")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.query = text.NewEditor("prior")
	options := document.SearchOptions{MatchCase: true, WholeWord: true, Regex: true}
	state.searchOptions = options
	dirty := doc.Dirty()
	state.command("Select and Find Next")
	if state.query.String() != "a.b" || state.searchOptions != options || doc.Editor.Selection() != (text.Range{Start: 8, End: 11}) ||
		doc.Editor.String() != "a.b axb a.b" || doc.Dirty() != dirty || doc.Editor.CanUndo() {
		t.Fatalf("selected literal find = query:%q options:%+v selection:%+v text:%q", state.query.String(), state.searchOptions, doc.Editor.Selection(), doc.Editor.String())
	}
	state.command("Select and Find Previous")
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) || state.query.String() != "a.b" {
		t.Fatalf("selected backward find = %+v query=%q", doc.Editor.Selection(), state.query.String())
	}
}

func TestSelectAndFindExpandsUnicodeWordAndGuardsThreshold(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("猫語 dog 猫語")
	doc.Editor.MoveTo(1, false)
	state.command("Select and Find Next")
	if state.query.String() != "猫語" || doc.Editor.Selection() != (text.Range{Start: 7, End: 9}) {
		t.Fatalf("forward current word = %q %+v", state.query.String(), doc.Editor.Selection())
	}
	doc.Editor.MoveTo(8, false)
	state.command("Select and Find Previous")
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 2}) {
		t.Fatalf("backward current word = %+v", doc.Editor.Selection())
	}
	long := state.space.NewDocument()
	long.Editor.Insert(strings.Repeat("x", 1025) + " y")
	long.Editor.SetSelection(text.Range{Start: 0, End: 1025})
	state.showInActivePane(long)
	state.query = text.NewEditor("prior")
	state.command("Select and Find Next")
	if state.query.String() != "prior" || long.Editor.Selection() != (text.Range{Start: 0, End: 1025}) || !strings.Contains(state.notice, "1024") {
		t.Fatalf("overlong selection changed search: query=%q selection=%+v notice=%q", state.query.String(), long.Editor.Selection(), state.notice)
	}
}

func TestVolatileFindForcesCaseInsensitiveNonWholeWrapWithoutChangingFindBar(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("Cat cat CAT catalog")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.query = text.NewEditor("other")
	options := document.SearchOptions{MatchCase: true, WholeWord: true, Regex: true}
	state.searchOptions = options
	dirty := doc.Dirty()
	state.command("Find (Volatile) Next")
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || state.query.String() != "other" || state.searchOptions != options {
		t.Fatalf("volatile next = %+v query=%q options=%+v", doc.Editor.Selection(), state.query.String(), state.searchOptions)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.command("Find (Volatile) Previous")
	if doc.Editor.Selection() != (text.Range{Start: 12, End: 15}) || state.query.String() != "other" ||
		doc.Editor.String() != "Cat cat CAT catalog" || doc.Dirty() != dirty || doc.Editor.CanUndo() {
		t.Fatalf("volatile backward wrap/nonwhole = %+v text=%q", doc.Editor.Selection(), doc.Editor.String())
	}
	doc.Editor.MoveTo(0, false)
	state.command("Find (Volatile) Next")
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 0}) || state.query.String() != "other" {
		t.Fatal("empty volatile selection changed editor or Find What")
	}
}

func TestSelectedSearchMenuAndF3ShortcutsAtCompactViewport(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("cat dog cat"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(0, 3); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == searchCommandID(t, "Selected Text Search") && hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
			t.Fatalf("selected-search menu overlaps status: %+v", hit.Bounds)
		}
	}
	clickID(t, driver, searchCommandID(t, "Selected Text Search"))
	for index := range selectedSearchCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == selectedSearchScope.At(index) {
				found = true
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("selected-search row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("selected-search row %d missing", index)
		}
	}
	clickID(t, driver, selectedSearchScope.At(0))
	if state.query.String() != "cat" || state.space.Current().Editor.Selection() != (text.Range{Start: 8, End: 11}) {
		t.Fatalf("menu selected-find = %q %+v", state.query.String(), state.space.Current().Editor.Selection())
	}
	state.space.Current().Editor.SetSelection(text.Range{Start: 0, End: 3})
	if err := driver.Key(gfx.KeyF3, primary()|gfx.ModifierAlt); err != nil {
		t.Fatal(err)
	}
	if state.space.Current().Editor.Selection() != (text.Range{Start: 8, End: 11}) || state.query.String() != "cat" {
		t.Fatal("volatile F3 shortcut did not search selection without changing query")
	}
	if err := driver.Key(gfx.KeyF3, primary()|gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if state.space.Current().Editor.Selection() != (text.Range{Start: 0, End: 3}) || state.query.String() != "cat" {
		t.Fatal("Select and Find Previous shortcut missed first match")
	}
}

func TestSelectedSearchOnlyMovesActiveSplitViewSelection(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left left")
	right := state.space.NewDocument()
	right.Editor.Insert("right right")
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	right.Editor.SetSelection(text.Range{Start: 0, End: 5})
	state.command("Select and Find Next")
	if left.Editor.String() != "left left" || left.Editor.Selection().Start != left.Editor.Selection().End ||
		right.Editor.Selection() != (text.Range{Start: 6, End: 11}) || state.query.String() != "right" {
		t.Fatalf("split selected-find = left:%+v right:%+v query=%q", left.Editor.Selection(), right.Editor.Selection(), state.query.String())
	}
}

func TestSelectedSearchEscapeKeepsExistingResults(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("cat cat")
	state.query = text.NewEditor("cat")
	state.command("Find All in Current Document")
	if !state.resultsOpen {
		t.Fatal("test results pane did not open")
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Selected Text Search"))
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.selectedSearchMenuOpen || !state.resultsOpen || state.menuOpen != 2 {
		t.Fatalf("Escape closed results instead of submenu: menu=%v results=%v parent=%d", state.selectedSearchMenuOpen, state.resultsOpen, state.menuOpen)
	}
}
