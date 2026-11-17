package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

// liveDocumentCompositionInput establishes the tentative capability explicitly
// before resolving the pane input. On non-Darwin the app never auto-enables it,
// so a fixture that only called documentCompositionInput would silently never
// establish a live composition there; production platform gating is unchanged.
func liveDocumentCompositionInput(t *testing.T, state *model, pane int) *app.TextInput {
	t.Helper()
	doc := state.paneDocument(pane)
	if !doc.Editor.TentativeCompositionEnabled() && !doc.Editor.EnableTentativeComposition() {
		t.Fatal("document editor refused tentative composition")
	}
	input := documentCompositionInput(t, state, pane)
	if !input.State.TentativeComposition {
		t.Fatalf("resolved input did not route the tentative capability: %+v", input.State)
	}
	return input
}

// TestProvisionalCompositionFreezesEditTrackedIndicators proves that a paint
// during a live tentative update does not advance or drop the edit-tracked
// indicator groups (fold markers, hidden lines, Find Mark spans, user styles)
// and that they settle exactly once on the terminal event. A provisional update
// is not committed source, so the app must not fabricate tracking primitives
// from the transient final text.
func TestProvisionalCompositionFreezesEditTrackedIndicators(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("a\nb\nc")
	doc.Editor.MoveTo(0, false)
	state.setBookmarkLines(doc, []int{0})
	state.folded[doc] = map[int]bool{0: true}
	state.foldCache[doc] = foldCache{value: doc.Editor.String(), mode: state.languageFor(doc)}
	state.hiddenLines[doc] = []hiddenLineRange{{Start: 1, End: 2}}
	state.hiddenValue[doc] = doc.Editor.String()
	state.markedDocument, state.markedValue = doc, doc.Editor.String()
	state.markedSpans = []text.Span{{Start: 0, End: 1}}
	state.markedTracking = trackIndicators(doc.Editor)
	state.styledMarks = map[*document.Document]*styleMarkState{
		doc: {tracking: trackIndicators(doc.Editor), styles: [5][]text.Span{{{Start: 0, End: 1}}}},
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}

	input := liveDocumentCompositionInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "x")
	if _, composing := doc.Editor.Composition(); !composing || state.provisional.doc != doc {
		t.Fatalf("indicator fixture lacks a live composition: composing=%v provisional=%+v", composing, state.provisional)
	}
	if doc.Editor.String() != "xa\nb\nc" {
		t.Fatalf("provisional text = %q", doc.Editor.String())
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.folded[doc][0] {
		t.Fatalf("paint dropped provisional fold state: %+v", state.folded[doc])
	}
	if got := state.hiddenLines[doc]; len(got) != 1 || got[0] != (hiddenLineRange{Start: 1, End: 2}) {
		t.Fatalf("paint dropped provisional hidden lines: %+v", got)
	}
	if stored := state.bookmarks[doc]; stored == nil || len(stored.lines) != 1 || stored.lines[0] != 0 {
		t.Fatalf("paint advanced provisional bookmarks: %+v", stored)
	}
	if got := state.markedSpans; len(got) != 1 || got[0] != (text.Span{Start: 0, End: 1}) {
		t.Fatalf("paint advanced provisional Find Mark span: %+v", got)
	}
	if stored := state.styledMarks[doc]; stored == nil || len(stored.styles[0]) != 1 || stored.styles[0][0] != (text.Span{Start: 0, End: 1}) {
		t.Fatalf("paint advanced provisional user style span: %+v", stored)
	}

	sendComposition(t, input, text.CompositionCommit, "")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "xa\nb\nc" || state.provisional.doc != nil {
		t.Fatalf("commit = %q provisional=%+v", doc.Editor.String(), state.provisional)
	}
	// The committed insertion before the tracked text settles both groups
	// exactly once, at the post-commit coordinates.
	if got := state.markedSpans; len(got) != 1 || got[0] != (text.Span{Start: 1, End: 2}) {
		t.Fatalf("committed Find Mark span = %+v", got)
	}
	if state.markedValue != doc.Editor.String() {
		t.Fatalf("committed Find Mark base = %q", state.markedValue)
	}
	if got := state.styleSpans(doc, 0); len(got) != 1 || got[0] != (text.Span{Start: 1, End: 2}) {
		t.Fatalf("committed user style span = %+v", got)
	}
}

// TestCompositionLifecyclePathsCancelWithoutStaleState covers the app paths
// that leave the document editor while a tentative is live: focus to no target,
// save, reload from disk and close. Each path must cancel the live tentative
// exactly once, leave the editor at its pre-composition text, and clear the app
// frame so a later paint cannot show provisional state on a retired owner.
func TestCompositionLifecyclePathsCancelWithoutStaleState(t *testing.T) {
	t.Run("focus to nil", func(t *testing.T) {
		state := newModel()
		doc := state.space.Current()
		doc.Editor.Insert("abc")
		input := liveDocumentCompositionInput(t, state, 0)
		sendComposition(t, input, text.CompositionUpdate, "x")
		if _, composing := doc.Editor.Composition(); !composing {
			t.Fatal("focus fixture lacks a live composition")
		}
		state.reconcileTextOwner(gfx.ID{}, nil)
		if doc.Editor.String() != "abc" {
			t.Fatalf("focus to nil rollback = %q", doc.Editor.String())
		}
		if _, composing := doc.Editor.Composition(); composing || state.provisional.doc != nil {
			t.Fatalf("focus to nil stranded composition: provisional=%+v", state.provisional)
		}
		if state.compositionOwner.editor != nil || !state.compositionOwner.target.IsZero() {
			t.Fatalf("focus to nil owner = %+v", state.compositionOwner)
		}
	})

	t.Run("save writes the settled preimage", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "save.txt")
		if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
			t.Fatal(err)
		}
		state := newModel()
		if err := state.space.Open(path); err != nil {
			t.Fatal(err)
		}
		state.showInActivePane(state.space.Current())
		doc := state.space.Current()
		doc.Editor.MoveTo(3, false)
		input := liveDocumentCompositionInput(t, state, 0)
		sendComposition(t, input, text.CompositionUpdate, "x")
		if _, composing := doc.Editor.Composition(); !composing {
			t.Fatal("save fixture lacks a live composition")
		}
		state.command("Save")
		stored, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(stored) != "abc" || doc.Editor.String() != "abc" {
			t.Fatalf("save wrote provisional text: file=%q editor=%q", stored, doc.Editor.String())
		}
		if state.provisional.doc != nil || doc.Dirty() {
			t.Fatalf("save left provisional=%+v dirty=%v", state.provisional, doc.Dirty())
		}
	})

	t.Run("reload cancels the retired editor", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "reload.txt")
		if err := os.WriteFile(path, []byte("abc\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		state := newModel()
		if err := state.space.Open(path); err != nil {
			t.Fatal(err)
		}
		state.showInActivePane(state.space.Current())
		previous := state.space.Current()
		previous.Editor.MoveTo(2, false)
		input := liveDocumentCompositionInput(t, state, 0)
		sendComposition(t, input, text.CompositionUpdate, "x")
		if _, composing := previous.Editor.Composition(); !composing {
			t.Fatal("reload fixture lacks a live composition")
		}
		current := state.reloadDocument(0, false)
		if current == nil || current == previous {
			t.Fatalf("reload = %+v", current)
		}
		if previous.Editor.String() != "abc\n" {
			t.Fatalf("retired reload editor = %q", previous.Editor.String())
		}
		if _, composing := previous.Editor.Composition(); composing || state.provisional.doc != nil {
			t.Fatalf("reload stranded composition: provisional=%+v", state.provisional)
		}
		if current.Editor.String() != "abc\n" {
			t.Fatalf("reloaded text = %q", current.Editor.String())
		}
		if state.compositionOwner.editor == previous.Editor {
			t.Fatalf("reload kept retired owner: %+v", state.compositionOwner)
		}
	})

	t.Run("close releases the composition frame", func(t *testing.T) {
		state := newModel()
		doc := state.space.Current()
		doc.Editor.Insert("abc")
		state.setBookmarkLines(doc, []int{0})
		state.markedDocument, state.markedValue = doc, doc.Editor.String()
		state.markedSpans = []text.Span{{Start: 0, End: 3}}
		input := liveDocumentCompositionInput(t, state, 0)
		sendComposition(t, input, text.CompositionUpdate, "x")
		if _, composing := doc.Editor.Composition(); !composing {
			t.Fatal("close fixture lacks a live composition")
		}
		state.pendingClose = 0
		state.finishClose(true)
		if state.pendingClose != -1 {
			t.Fatalf("pendingClose = %d", state.pendingClose)
		}
		if doc.Editor.String() != "abc" {
			t.Fatalf("closed editor rollback = %q", doc.Editor.String())
		}
		if _, composing := doc.Editor.Composition(); composing || state.provisional.doc != nil {
			t.Fatalf("close stranded composition: provisional=%+v", state.provisional)
		}
		if state.compositionOwner.editor != nil || state.bookmarks[doc] != nil || state.markedDocument == doc {
			t.Fatalf("close kept app state: owner=%+v bookmarks=%+v marked=%v", state.compositionOwner, state.bookmarks[doc], state.markedDocument == doc)
		}
	})
}
