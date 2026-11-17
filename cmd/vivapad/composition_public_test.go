package main

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestPublicCompositionDefersBookmarkMappingUntilResult(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.setBookmarkLines(doc, []int{0})
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "\nx"}); err != nil {
		t.Fatal(err)
	}
	if stored := state.bookmarks[doc]; stored.value != "a\nb" || !slices.Equal(stored.lines, []int{0}) {
		t.Fatalf("preedit changed the committed bookmark basis: text=%q lines=%v", stored.value, stored.lines)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "\n中"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "\n中a\nb" || !slices.Equal(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("result failed to map the bookmark once: text=%q state=%+v", doc.Editor.String(), state.bookmarks[doc])
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\nb" {
		t.Fatalf("result lost the ordinary Undo unit: %q", doc.Editor.String())
	}
}

func TestPublicCompositionEmptyUpdateThenDirectResult(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 2, End: 1})
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate}); err != nil {
		t.Fatal(err)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "a中" || !doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatalf("direct Result did not replace the selected text: %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab" {
		t.Fatalf("direct Result damaged selected Undo: %q", doc.Editor.String())
	}
}

func TestPublicCompositionRejectedResultPreservesPreedit(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.setBookmarkLines(doc, []int{0})
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "\nx"}); err != nil {
		t.Fatal(err)
	}
	before := app.TextStateOf(doc.Editor)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: string([]byte{0xff})}); err != nil {
		t.Fatal(err)
	}
	if got := app.TextStateOf(doc.Editor); got != before || state.provisional.doc != doc || state.bookmarks[doc].value != "a\nb" {
		t.Fatalf("rejected Result changed preedit or committed metadata: before=%+v after=%+v", before, got)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "\n中"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "\n中a\nb" || !slices.Equal(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("retry did not commit once: text=%q lines=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestPublicCompositionCancelKeepsOrdinarySelectionDeletionWithoutMacro(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	doc.Editor.SetSavePoint()
	doc.Editor.SetSelection(text.Range{Start: 4, End: 1})
	state.setBookmarkLines(doc, []int{1})
	state.macroRecording, state.macroSource = true, doc
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "你"}); err != nil {
		t.Fatal(err)
	}
	if len(state.macroSteps) != 0 || state.provisional.doc != doc {
		t.Fatal("preedit leaked into the macro or lost ownership")
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextCancelComposition, CompositionPhase: text.CompositionCancel}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "ad" || doc.Editor.AtSavePoint() || !doc.Editor.CanUndo() ||
		state.macroRecording || len(state.macroSteps) != 0 || state.provisional.doc != nil || state.bookmarks[doc].value != "ad" ||
		!slices.Equal(state.bookmarks[doc].lines, []int{0}) {
		t.Fatalf("Cancel lost the ordinary selection deletion or marker mapping: text=%q bookmark=%+v", doc.Editor.String(), state.bookmarks[doc])
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd" {
		t.Fatalf("ordinary deletion was not Undoable after Cancel: %q", doc.Editor.String())
	}
}

func TestPublicCompositionEmptyUpdateKeepsOrdinarySelectionDeletion(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 4, End: 1})
	state.setBookmarkLines(doc, []int{1})
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "你"}); err != nil {
		t.Fatal(err)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "ad" || state.provisional.doc != nil || state.bookmarks[doc].value != "ad" ||
		!slices.Equal(state.bookmarks[doc].lines, []int{0}) {
		t.Fatalf("empty Update did not settle the ordinary deletion: text=%q bookmark=%+v", doc.Editor.String(), state.bookmarks[doc])
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd" {
		t.Fatalf("empty Update lost the deletion Undo: %q", doc.Editor.String())
	}
}

func TestPublicCompositionFocusHandoffCancelsOldDocument(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("before")
	doc.Editor.ClearHistory()
	doc.Editor.SetSavePoint()
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	provider := newConfig(state).TextInput
	input := provider(app.TextInputContext{ID: editorID})
	if input == nil {
		t.Fatal("document input unavailable")
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "你"}); err != nil {
		t.Fatal(err)
	}
	if state.provisional.doc != doc {
		t.Fatal("document did not retain the live preedit")
	}
	if query := provider(app.TextInputContext{ID: queryID}); query == nil || state.provisional.doc != nil ||
		doc.Editor.String() != "before" || !doc.Editor.AtSavePoint() {
		t.Fatalf("focus handoff retained or committed old preedit: text=%q provisional=%+v", doc.Editor.String(), state.provisional)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "before" {
		t.Fatalf("stale Result edited the unfocused document: %q", doc.Editor.String())
	}
}

func TestPublicCompositionPointerAwayCancelsBeforeProviderHandoff(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("before")
	doc.Editor.ClearHistory()
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	config := newConfig(state)
	input := config.TextInput(app.TextInputContext{ID: editorID})
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "你"}); err != nil {
		t.Fatal(err)
	}
	config.OnPointer(gfx.PointerEvent{Kind: gfx.PointerDown, Target: queryID})
	if doc.Editor.String() != "before" || state.provisional.doc != nil {
		t.Fatalf("pointer handoff left a live preedit: %q provisional=%+v", doc.Editor.String(), state.provisional)
	}
}

func TestPublicCompositionCloneTargetHandoffRejectsOldResult(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("before")
	doc.Editor.ClearHistory()
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{doc, doc}
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	provider := newConfig(state).TextInput
	left := provider(app.TextInputContext{ID: editorID})
	if _, err := left.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "你"}); err != nil {
		t.Fatal(err)
	}
	if right := provider(app.TextInputContext{ID: rightEditorID}); right == nil ||
		doc.Editor.String() != "before" || state.provisional.doc != nil {
		t.Fatalf("same-editor target change retained old tentative: %q", doc.Editor.String())
	}
	if _, err := left.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "旧"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "before" {
		t.Fatalf("old pane Result reached shared editor: %q", doc.Editor.String())
	}
}

func TestPublicCompositionOldProviderCannotReviveAfterTargetReturns(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("before")
	doc.Editor.ClearHistory()
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{doc, doc}
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	provider := newConfig(state).TextInput
	oldLeft := provider(app.TextInputContext{ID: editorID})
	if _, err := oldLeft.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "你"}); err != nil {
		t.Fatal(err)
	}
	provider(app.TextInputContext{ID: rightEditorID})
	provider(app.TextInputContext{ID: editorID})
	if _, err := oldLeft.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "旧"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "before" || state.provisional.doc != nil {
		t.Fatalf("old provider Result revived after focus returned: %q", doc.Editor.String())
	}
}

func TestPublicCompositionReadOnlyRefusalCancelsPendingInput(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("before")
	doc.Editor.ClearHistory()
	doc.Editor.SetSavePoint()
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := newConfig(state).TextInput(app.TextInputContext{ID: editorID})
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "你"}); err != nil {
		t.Fatal(err)
	}
	state.readOnly[doc] = true
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "before" || state.provisional.doc != nil || !doc.Editor.AtSavePoint() {
		t.Fatalf("read-only refusal kept preedit or committed text: %q provisional=%+v", doc.Editor.String(), state.provisional)
	}
}

func TestPublicCompositionExplicitResultKeepsEarlierSelectAnchor(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		before string
		want   string
		event  app.TextEvent
	}{
		{name: "rune range", before: "aaaa", want: "aaa中", event: app.TextEvent{
			Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中",
			Replacement: app.TextRange{Start: 3, End: 4}, HasReplacement: true,
		}},
		{name: "UTF-16 range", before: "a😀bc", want: "a😀b中", event: app.TextEvent{
			Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中",
			ReplacementUTF16: text.UTF16Range{Start: 4, Length: 1}, HasReplacementUTF16: true,
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert(scenario.before)
			doc.Editor.ClearHistory()
			doc.Editor.MoveTo(0, false)
			state.selectAnchors[0] = selectAnchor{doc: doc, index: 2, value: scenario.before}
			if !doc.Editor.EnableTentativeComposition() {
				t.Fatal("published Viva rejected the document composition capability")
			}
			input := state.macroTextInput(doc)
			if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
				t.Fatal(err)
			}
			if _, err := input.OnEvent(scenario.event); err != nil {
				t.Fatal(err)
			}
			if got := state.selectAnchors[0]; doc.Editor.String() != scenario.want || got.index != 2 || got.value != scenario.want {
				t.Fatalf("explicit Result moved unaffected anchor: text=%q anchor=%+v", doc.Editor.String(), got)
			}
		})
	}
}

func TestPublicCompositionExplicitResultCancelsUnreplayableMacro(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("aaaa")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.macroRecording, state.macroSource = true, doc
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中",
		Replacement: app.TextRange{Start: 3, End: 4}, HasReplacement: true}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "aaa中" || state.macroRecording || len(state.macroSteps) != 0 {
		t.Fatalf("unreplayable replacement remained in recording: text=%q steps=%+v", doc.Editor.String(), state.macroSteps)
	}
}

func TestPublicCompositionSelectedExplicitResultMapsBothEdits(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("abXcd")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 1, End: 2})
	state.selectAnchors[0] = selectAnchor{doc: doc, index: 3, value: doc.Editor.String()}
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中",
		Replacement: app.TextRange{Start: 3, End: 4}, HasReplacement: true}); err != nil {
		t.Fatal(err)
	}
	if anchor := state.selectAnchors[0]; doc.Editor.String() != "aXc中" || anchor.index != 2 || anchor.value != "aXc中" {
		t.Fatalf("selected deletion and explicit result mis-mapped anchor: text=%q anchor=%+v", doc.Editor.String(), anchor)
	}
}

func TestPublicCompositionNoOpEventsKeepMacroRecording(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		event app.TextEvent
	}{
		{name: "empty update", event: app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate}},
		{name: "inactive cancel", event: app.TextEvent{Kind: app.TextCancelComposition, CompositionPhase: text.CompositionCancel}},
		{name: "inactive commit", event: app.TextEvent{Kind: app.TextUnmark, CompositionPhase: text.CompositionCommit}},
		{name: "empty explicit result", event: app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult,
			Replacement: app.TextRange{Start: 1, End: 1}, HasReplacement: true}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert("ab")
			doc.Editor.ClearHistory()
			doc.Editor.SetSelection(text.Range{Start: 1, End: 2})
			selection := doc.Editor.Selection()
			state.macroRecording, state.macroSource = true, doc
			if !doc.Editor.EnableTentativeComposition() {
				t.Fatal("published Viva rejected the document composition capability")
			}
			input := state.macroTextInput(doc)
			if _, err := input.OnEvent(scenario.event); err != nil {
				t.Fatal(err)
			}
			if doc.Editor.String() != "ab" || doc.Editor.Selection() != selection || !state.macroRecording || len(state.macroSteps) != 0 {
				t.Fatalf("no-op tagged event canceled macro or edited selection: text=%q selection=%+v recording=%v", doc.Editor.String(), doc.Editor.Selection(), state.macroRecording)
			}
		})
	}
}

func TestPublicCompositionExplicitUpdateMapsItsOwnDeletionSite(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("abXcd")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 1, End: 2})
	state.selectAnchors[0] = selectAnchor{doc: doc, index: 2, value: doc.Editor.String()}
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p",
		Replacement: app.TextRange{Start: 3, End: 4}, HasReplacement: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中",
		Replacement: app.TextRange{Start: 3, End: 4}, HasReplacement: true}); err != nil {
		t.Fatal(err)
	}
	if anchor := state.selectAnchors[0]; doc.Editor.String() != "abX中" || anchor.index != 2 || anchor.value != "abX中" {
		t.Fatalf("explicit Update mapped deletion from selection instead: text=%q anchor=%+v", doc.Editor.String(), anchor)
	}
}

func TestPublicCompositionRolledBackEmptyResultKeepsMacroRecording(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	state.macroRecording, state.macroSource = true, doc
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult,
		Replacement: app.TextRange{Start: 1, End: 1}, HasReplacement: true}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "ab" || !state.macroRecording || len(state.macroSteps) != 0 {
		t.Fatalf("rolled-back no-op canceled recording: text=%q recording=%v steps=%+v", doc.Editor.String(), state.macroRecording, state.macroSteps)
	}
}

func TestPublicCompositionDirectSelectedResultCancelsMacro(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 1, End: 2})
	state.macroRecording, state.macroSource = true, doc
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "a中" || state.macroRecording || len(state.macroSteps) != 0 {
		t.Fatalf("selected Result produced an unreplayable macro: text=%q recording=%v steps=%+v", doc.Editor.String(), state.macroRecording, state.macroSteps)
	}
}

func TestPublicCompositionNetEqualSeparatedEditsMoveAnchor(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("aaaa")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	state.selectAnchors[0] = selectAnchor{doc: doc, index: 2, value: doc.Editor.String()}
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "a",
		Replacement: app.TextRange{Start: 3, End: 3}, HasReplacement: true}); err != nil {
		t.Fatal(err)
	}
	if anchor := state.selectAnchors[0]; doc.Editor.String() != "aaaa" || anchor.index != 1 || anchor.value != "aaaa" {
		t.Fatalf("net-equal separated edits lost anchor identity: text=%q anchor=%+v", doc.Editor.String(), anchor)
	}
}
