package main

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
)

func TestPublicMigrationEmptyFirstPreeditSelectedResult(t *testing.T) {
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
		t.Fatalf("direct Result did not replace selection once: %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab" {
		t.Fatalf("selected Result damaged ordinary Undo: %q", doc.Editor.String())
	}
}

func TestPublicMigrationEmptyUpdateLeavesNoGhostFrame(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 1, End: 2})
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("published Viva rejected the document composition capability")
	}
	input := state.macroTextInput(doc)
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.CompositionActive() || state.provisional.doc != nil || doc.Editor.String() != "ab" || doc.Editor.CanUndo() {
		t.Fatalf("empty Update left a ghost frame or history: active=%v frame=%+v text=%q", doc.Editor.CompositionActive(), state.provisional, doc.Editor.String())
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.CompositionActive() || doc.Editor.String() != "a中" || !doc.Editor.Undo() || doc.Editor.String() != "ab" || doc.Editor.CanUndo() {
		t.Fatalf("empty Update/Result did not commit once: active=%v text=%q", doc.Editor.CompositionActive(), doc.Editor.String())
	}
}

func TestPublicMigrationNetEqualSeparatedEditsMoveAnchor(t *testing.T) {
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

func TestPublicMigrationSelectedExplicitResultMapsBothEdits(t *testing.T) {
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
		t.Fatalf("selected deletion and explicit Result mis-mapped anchor: text=%q anchor=%+v", doc.Editor.String(), anchor)
	}
}

func TestPublicMigrationUnreplayableCompositionCancelsMacro(t *testing.T) {
	for _, scenario := range []struct {
		name, original, want string
		selectText           bool
		explicit             bool
	}{
		{name: "explicit replacement", original: "aaaa", want: "aaa中", explicit: true},
		{name: "direct selected result", original: "ab", want: "a中", selectText: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert(scenario.original)
			doc.Editor.ClearHistory()
			if scenario.selectText {
				doc.Editor.SetSelection(text.Range{Start: 1, End: 2})
			} else {
				doc.Editor.MoveTo(0, false)
			}
			state.macroRecording, state.macroSource = true, doc
			if !doc.Editor.EnableTentativeComposition() {
				t.Fatal("published Viva rejected the document composition capability")
			}
			input := state.macroTextInput(doc)
			if scenario.explicit {
				if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
					t.Fatal(err)
				}
			}
			result := app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "中"}
			if scenario.explicit {
				result.Replacement, result.HasReplacement = app.TextRange{Start: 3, End: 4}, true
			}
			if _, err := input.OnEvent(result); err != nil {
				t.Fatal(err)
			}
			if doc.Editor.String() != scenario.want || state.macroRecording || len(state.macroSteps) != 0 {
				t.Fatalf("unreplayable Result remained in macro: text=%q recording=%v steps=%+v", doc.Editor.String(), state.macroRecording, state.macroSteps)
			}
		})
	}
}

func TestPublicMigrationRolledBackNoOpKeepsMacroRecording(t *testing.T) {
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
		t.Fatalf("rolled-back no-op canceled macro: text=%q recording=%v steps=%+v", doc.Editor.String(), state.macroRecording, state.macroSteps)
	}
}
