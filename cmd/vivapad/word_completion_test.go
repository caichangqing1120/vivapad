package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestWordCompletionSingleCandidateIsOneUndoUnitAndKeepsBookmark(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("alphabet\nalp")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	state.command("Toggle Bookmark")
	state.command("Word Completion")
	if doc.Editor.String() != "alphabet\nalphabet" || state.wordCompletion != nil || !doc.Dirty() || len(state.bookmarkLines(doc)) != 1 {
		t.Fatalf("single-candidate completion = %q, popup=%+v, marks=%v", doc.Editor.String(), state.wordCompletion, state.bookmarkLines(doc))
	}
	state.command("Undo")
	if doc.Editor.String() != "alphabet\nalp" || len(state.bookmarkLines(doc)) != 1 {
		t.Fatalf("completion undo = %q, marks=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestWordCompletionPopupKeyboardAndPointerAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("alpha alphabet alp"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, primary()); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion == nil || len(state.wordCompletion.items) != 2 {
		t.Fatalf("primary+Enter did not open candidates: %+v", state.wordCompletion)
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID.Scope() == wordCompletionScope && hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
			t.Fatalf("completion row overlaps status: %+v", hit.Bounds)
		}
	}
	if err := driver.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion == nil {
		t.Fatalf("Down dismissed completion: caret=%d selection=%+v focus=%v", state.space.Current().Editor.Caret(), state.space.Current().Editor.Selection(), driver.Snapshot().Focused)
	}
	if state.wordCompletion.selected != 1 {
		t.Fatalf("Down selected %d, caret=%d focus=%v", state.wordCompletion.selected, state.space.Current().Editor.Caret(), driver.Snapshot().Focused)
	}
	if !state.validWordCompletion() {
		t.Fatalf("Down invalidated completion: caret=%d expected=%d source=%q current=%q focus=%v", state.space.Current().Editor.Caret(), state.wordCompletion.end, state.wordCompletion.source, state.space.Current().Editor.String(), driver.Snapshot().Focused)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion != nil || state.space.Current().Editor.String() != "alpha alphabet alphabet" {
		t.Fatalf("keyboard completion = %q, popup=%+v, notice=%q, caret=%d, focus=%v", state.space.Current().Editor.String(), state.wordCompletion, state.notice, state.space.Current().Editor.Caret(), driver.Snapshot().Focused)
	}
	state.command("Undo")
	if state.space.Current().Editor.String() != "alpha alphabet alp" {
		t.Fatal("completion Undo did not restore the typed prefix")
	}
	state.space.Current().Editor.MoveTo(state.space.Current().Editor.RuneCount(), false)
	state.command("Word Completion")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, wordCompletionScope.At(0))
	if got := state.space.Current().Editor.String(); got != "alpha alphabet alpha" {
		t.Fatalf("pointer completion = %q", got)
	}
}

func TestWordCompletionRejectsReadOnlyIMEAndStalePopup(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("alpha alphabet alp")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	state.command("Read-Only on Current Document")
	state.command("Word Completion")
	if state.wordCompletion != nil || doc.Editor.String() != "alpha alphabet alp" || !strings.Contains(state.notice, "Read-only") {
		t.Fatalf("read-only completion = %+v, %q", state.wordCompletion, state.notice)
	}
	state.command("Clear Read-Only for All Documents")
	state.command("Word Completion")
	if state.wordCompletion == nil {
		t.Fatal("expected multiple candidates before stale edit")
	}
	doc.Editor.Insert("z")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion != nil || doc.Editor.String() != "alpha alphabet alpz" {
		t.Fatal("stale candidate survived an external edit")
	}
	doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: doc.Editor.RuneCount()})
	doc.Editor.SetMarkedText("al", text.Range{Start: 2, End: 2})
	before := doc.Editor.String()
	state.command("Word Completion")
	if state.wordCompletion != nil || doc.Editor.String() != before {
		t.Fatal("completion interrupted active IME composition")
	}
}

func TestWordCompletionMenuAndSplitPaneScope(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	left := state.space.Current()
	right := state.space.NewDocument()
	right.Editor.Insert("alpha alphabet alp")
	right.Editor.MoveTo(right.Editor.RuneCount(), false)
	state.toggleSplit()
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(18))
	clickID(t, driver, autoCompletionScope.At(1))
	if state.wordCompletion == nil || state.wordCompletion.doc != right || state.wordCompletion.pane != 1 {
		t.Fatalf("menu completion used wrong pane: %+v", state.wordCompletion)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion != nil || left.Editor.String() != "" || right.Editor.String() != "alpha alphabet alp" {
		t.Fatal("Escape did not dismiss without changing split documents")
	}
}

func TestWordCompletionDismissesAfterTabOrCaretChange(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("alpha alphabet alp")
	state.command("Word Completion")
	if state.wordCompletion == nil {
		t.Fatal("expected candidates")
	}
	state.showInActivePane(state.space.NewDocument())
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion != nil || doc.Editor.String() != "alpha alphabet alp" {
		t.Fatal("tab switch retained stale completion")
	}
	state.space.Select(state.documentIndex(doc))
	state.showInActivePane(doc)
	state.command("Word Completion")
	if state.wordCompletion == nil {
		t.Fatal("expected candidates after returning")
	}
	doc.Editor.MoveLeft(false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion != nil || doc.Editor.String() != "alpha alphabet alp" {
		t.Fatal("caret movement retained stale completion")
	}
}

func TestWordCompletionPopupStaysInsideCompactViewportNearBottom(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("alpha alphabet\n" + strings.Repeat("line\n", 18) + "alp"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, primary()); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion == nil {
		t.Fatal("bottom-of-document completion did not open")
	}
	caret, ok := state.codeEditor().CaretBounds()
	if !ok {
		t.Fatal("caret missing")
	}
	found := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID.Scope() == wordCompletionScope {
			found = true
			if hit.Bounds.Y < menuHeight+toolbarHeight+tabHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight ||
				caret.Y > 300 && hit.Bounds.Y >= caret.Y {
				t.Fatalf("completion popup is clipped or below low caret: caret=%+v hit=%+v", caret, hit.Bounds)
			}
		}
	}
	if !found {
		t.Fatal("completion options not visible")
	}
}

func TestWordCompletionUsesLanguageCaseModeWithoutEditingNoMatch(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("Alpha alp")
	state.command("Word Completion")
	if doc.Editor.String() != "Alpha alp" || state.wordCompletion != nil || state.notice != "No word completion" {
		t.Fatalf("normal-text completion ignored case: %q, %q", doc.Editor.String(), state.notice)
	}
	state.languages[doc] = ".go"
	state.command("Word Completion")
	if doc.Editor.String() != "Alpha Alpha" || state.wordCompletion != nil {
		t.Fatalf("code completion missed case-insensitive candidate: %q", doc.Editor.String())
	}
}

func TestWordCompletionFiltersWhileTypingAndEnterDoesNotInsertNewline(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("alpha alphabet alp"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, primary()); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion == nil {
		t.Fatal("word list did not open")
	}
	if err := driver.Type("ha"); err != nil {
		t.Fatal(err)
	}
	if current := state.wordCompletion; current == nil || len(current.items) != 1 || current.items[0] != "alphabet" ||
		state.space.Current().Editor.String() != "alpha alphabet alpha" {
		t.Fatalf("typed filter = %+v, text=%q", current, state.space.Current().Editor.String())
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion != nil || state.space.Current().Editor.String() != "alpha alphabet alphabet" {
		t.Fatalf("native Enter did not accept completion: popup=%+v text=%q", state.wordCompletion, state.space.Current().Editor.String())
	}
}

func TestWordCompletionPopupMovesBeginEndSelectAnchor(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("alpha alphabet alp")
	state.command("Begin/End Select")
	state.command("Word Completion")
	if state.wordCompletion == nil {
		t.Fatal("completion list did not open")
	}
	state.acceptWordCompletion(1)
	if got := state.selectAnchors[0]; got.doc != doc || got.index != len([]rune("alpha alphabet alphabet")) || got.value != doc.Editor.String() {
		t.Fatalf("completion left selection anchor stale: %+v", got)
	}
}
