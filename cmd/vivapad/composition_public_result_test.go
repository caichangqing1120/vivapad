package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/macro"
)

func TestPublicCompositionDirectResultPreservesHistory(t *testing.T) {
	for _, test := range []struct {
		name, disk, original, before, want string
		labels                             string
		selection                          text.Range
		event                              app.TextEvent
		caret                              int
		undo                               []string
	}{
		{
			name: "caret", disk: "A\r\nB\nC", original: "A\nB\nC", before: "A\nB\nC!", want: "A\nB\nC!Z\n", labels: "WLW",
			selection: text.Range{Start: 6, End: 6}, caret: 8,
			event: app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "Z\n"},
			undo:  []string{"A\nB\nC!", "A\nB\nC"},
		},
		{
			name: "selected", disk: "A\r\nB\nC", original: "A\nB\nC", before: "A\nB\nC!", want: "A\nZ\n\nC!", labels: "WWL",
			selection: text.Range{Start: 3, End: 2}, caret: 4,
			event: app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "Z\n"},
			undo:  []string{"A\nZ\nC!", "A\nB\nC!", "A\nB\nC"},
		},
		{
			name: "raw UTF16", disk: "A\U0001d11e\r\nB\nC", original: "A\U0001d11e\nB\nC", before: "A\U0001d11e\nB\nC!", want: "A\U0001d11e\nZ\n\nC!", labels: "WWL",
			selection: text.Range{Start: 0, End: 0}, caret: 5,
			event: app.TextEvent{
				Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "Z\n",
				Replacement: app.TextRange{Start: 0, End: 1}, HasReplacement: true,
				ReplacementUTF16: text.UTF16Range{Start: 4, Length: 1}, HasReplacementUTF16: true,
			},
			undo: []string{"A\U0001d11e\n\nC!", "A\U0001d11e\nB\nC!", "A\U0001d11e\nB\nC"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "mixed.txt")
			if err := os.WriteFile(file, []byte(test.disk), 0o600); err != nil {
				t.Fatal(err)
			}
			state := newStartupModel([]string{file}, filepath.Join(root, "config", "session.json"))
			doc := state.space.Current()
			if doc.Editor.String() != test.original || doc.EOLSequence() != "WL" {
				t.Fatalf("mixed EOL fixture = %q labels=%q", doc.Editor.String(), doc.EOLSequence())
			}
			doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
			doc.Editor.Insert("!")
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			doc.Editor.SetSelection(test.selection)
			input := tentativeTextInput(t, state, 0)
			state.startMacro()
			next, err := input.OnEvent(test.event)
			if err != nil {
				t.Fatal(err)
			}
			if next.Value != test.want || doc.Editor.String() != test.want || next.Selection != (app.TextRange{Start: test.caret, End: test.caret}) || next.Composing || state.provisional.doc != nil {
				t.Fatalf("direct Result = %+v text=%q provisional=%+v", next, doc.Editor.String(), state.provisional)
			}
			if doc.EOLSequence() != test.labels || !doc.Dirty() || doc.Editor.AtSavePoint() {
				t.Fatalf("direct Result labels/save: labels=%q dirty=%v saved=%v", doc.EOLSequence(), doc.Dirty(), doc.Editor.AtSavePoint())
			}
			wantSteps := []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "Z\n"}}
			if !reflect.DeepEqual(state.macroSteps, wantSteps) {
				t.Fatalf("direct Result macro = %+v", state.macroSteps)
			}
			for _, want := range test.undo {
				if !doc.Editor.Undo() || doc.Editor.String() != want || doc.EOLSequence() != "WL" || doc.Dirty() != (want != test.before) {
					t.Fatalf("Undo = %q labels=%q dirty=%v; want %q", doc.Editor.String(), doc.EOLSequence(), doc.Dirty(), want)
				}
			}
			if doc.Editor.CanUndo() || doc.Editor.String() != test.original {
				t.Fatalf("direct Result fabricated history: %q", doc.Editor.String())
			}
			for index := len(test.undo) - 2; index >= -1; index-- {
				want, labels := test.want, test.labels
				if index >= 0 {
					want, labels = test.undo[index], "WL"
				}
				if !doc.Editor.Redo() || doc.Editor.String() != want || doc.EOLSequence() != labels || doc.Dirty() != (want != test.before) {
					t.Fatalf("Redo = %q labels=%q dirty=%v; want %q", doc.Editor.String(), doc.EOLSequence(), doc.Dirty(), want)
				}
			}
			if doc.Editor.CanRedo() {
				t.Fatal("direct Result left extra redo units")
			}
		})
	}
}

func TestPublicCompositionEmptyUpdateRetainsPendingResult(t *testing.T) {
	for _, phase := range []text.CompositionPhase{text.CompositionUpdate, text.CompositionCancel} {
		t.Run(map[text.CompositionPhase]string{text.CompositionUpdate: "empty update", text.CompositionCancel: "cancel then fresh result"}[phase], func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert("ab")
			doc.Editor.ClearHistory()
			doc.Editor.Insert("!")
			doc.Editor.BreakUndoGroup()
			doc.Editor.MoveTo(1, false)
			input := tentativeTextInput(t, state, 0)
			state.startMacro()
			sendComposition(t, input, text.CompositionUpdate, "pre")
			assertLiveComposition(t, doc)
			next := sendComposition(t, input, phase, "")
			if next.Value != "ab!" || next.Composing || state.provisional.doc != nil || len(state.macroSteps) != 0 {
				t.Fatalf("empty boundary retained provisional text: %+v frame=%+v macro=%+v", next, state.provisional, state.macroSteps)
			}
			next = sendComposition(t, input, text.CompositionResult, "Z")
			if next.Value != "aZb!" || next.Selection != (app.TextRange{Start: 2, End: 2}) || next.Composing || state.provisional.doc != nil {
				t.Fatalf("pending native Result lost: %+v frame=%+v", next, state.provisional)
			}
			if len(state.macroSteps) != 1 || state.macroSteps[0].Text != "Z" {
				t.Fatalf("Result macro = %+v", state.macroSteps)
			}
			for _, want := range []string{"ab!", "ab"} {
				if !doc.Editor.Undo() || doc.Editor.String() != want {
					t.Fatalf("Result Undo = %q; want %q", doc.Editor.String(), want)
				}
			}
			if doc.Editor.CanUndo() {
				t.Fatal("empty update added an undo unit")
			}
		})
	}
}

func TestPublicCompositionDirectResultUsesRectangleRows(t *testing.T) {
	state, driver := startDriver(t)
	doc := beginTrackedCompositionRectangle(t, state, driver)
	input := documentCompositionInput(t, state, 0)
	state.startMacro()
	next := sendComposition(t, input, text.CompositionResult, "Z")
	if next.Value != "aZb\ncZdQ" || next.Composing || state.provisional.doc != nil || !state.rectangles[0].active {
		t.Fatalf("direct rectangle Result = %+v rectangle=%+v", next, state.rectangles[0])
	}
	if state.macroRecording || len(state.macroSteps) != 0 {
		t.Fatalf("unsupported rectangle recorded macro: %+v", state.macroSteps)
	}
	for _, want := range []string{"ab\ncdQ", "ab\ncd"} {
		if !doc.Editor.Undo() || doc.Editor.String() != want {
			t.Fatalf("rectangle Result Undo = %q; want %q", doc.Editor.String(), want)
		}
	}
	if doc.Editor.CanUndo() {
		t.Fatal("rectangle Result added extra undo units")
	}
}

func TestPublicCompositionDirectResultFallsBackFromMultiSelection(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a a")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	state.multiSelectAll(document.SearchOptions{})
	input := tentativeTextInput(t, state, 0)
	state.startMacro()
	next := sendComposition(t, input, text.CompositionResult, "Z")
	if next.Value != "a Z" || state.multiSelections[0].active || next.Composing || state.provisional.doc != nil {
		t.Fatalf("direct multi Result = %+v multi=%+v", next, state.multiSelections[0])
	}
	if len(state.macroSteps) != 1 || state.macroSteps[0].Text != "Z" {
		t.Fatalf("multi fallback macro = %+v", state.macroSteps)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a a" {
		t.Fatalf("multi fallback Undo = %q; want a a", doc.Editor.String())
	}
	if doc.Editor.CanUndo() {
		t.Fatal("multi fallback duplicated its edit")
	}
}

func TestPublicCompositionDirectResultRefusalKeepsRectangle(t *testing.T) {
	for _, event := range []app.TextEvent{
		{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: string([]byte{0xff})},
		{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "Z", HasReplacementUTF16: true, ReplacementUTF16: text.UTF16Range{Start: -1, Length: 1}},
		{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "Z", HasReplacement: true, Replacement: app.TextRange{Start: -1, End: 1}},
	} {
		state, driver := startDriver(t)
		doc := beginTrackedCompositionRectangle(t, state, driver)
		input := documentCompositionInput(t, state, 0)
		state.startMacro()
		before, rectangle := captureComposition(state, doc), state.rectangles[0]
		if _, err := input.OnEvent(event); err != nil {
			t.Fatal(err)
		}
		if after := captureComposition(state, doc); !reflect.DeepEqual(after, before) || !reflect.DeepEqual(state.rectangles[0], rectangle) {
			t.Fatalf("invalid direct Result changed state: before=%+v after=%+v rectangle=%+v", before, after, state.rectangles[0])
		}
	}
}

func TestPublicCompositionInactiveCommitAndCancelPreserveHistory(t *testing.T) {
	state, doc, input := liveCompositionState(t)
	sendComposition(t, input, text.CompositionCancel, "")
	before := captureComposition(state, doc)
	for _, phase := range []text.CompositionPhase{text.CompositionCommit, text.CompositionCancel} {
		sendComposition(t, input, phase, "")
		if after := captureComposition(state, doc); !reflect.DeepEqual(before, after) {
			t.Fatalf("inactive terminal changed state: before=%+v after=%+v", before, after)
		}
	}
}

func TestPublicCompositionStaleRectangleResultPreservesProvenance(t *testing.T) {
	for _, test := range []struct {
		name, before, want string
		undo               []string
	}{
		{"inactive", "ab\ncdQ", "Zb\ncdQ", []string{"b\ncdQ", "ab\ncdQ", "ab\ncd"}},
		{"different document", "ab\ncdQ", "Zb\ncdQ", []string{"b\ncdQ", "ab\ncdQ", "ab\ncd"}},
		{"changed source", "ab\ncdQ!", "Zb\ncdQ!", []string{"b\ncdQ!", "ab\ncdQ!", "ab\ncdQ", "ab\ncd"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := beginTrackedCompositionRectangle(t, state, driver)
			input := documentCompositionInput(t, state, 0)
			switch test.name {
			case "inactive":
				state.rectangles[0].active = false
			case "different document":
				state.rectangles[0].doc = nil
			case "changed source":
				doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
				doc.Editor.BreakUndoGroup()
				doc.Editor.Insert("!")
			}
			if doc.Editor.String() != test.before {
				t.Fatalf("stale rectangle fixture = %q; want %q", doc.Editor.String(), test.before)
			}
			state.startMacro()
			next, err := input.OnEvent(app.TextEvent{
				Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "Z",
				Replacement: app.TextRange{Start: 3, End: 4}, HasReplacement: true,
				ReplacementUTF16: text.UTF16Range{Start: 0, Length: 1}, HasReplacementUTF16: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if next.Value != test.want || next.Selection != (app.TextRange{Start: 1, End: 1}) || state.rectangles[0].active || state.rectangles[0].doc != nil {
				t.Fatalf("stale rectangle lost Result provenance: next=%+v rectangle=%+v; want %q", next, state.rectangles[0], test.want)
			}
			if !state.macroRecording || len(state.macroSteps) != 1 || state.macroSteps[0].Text != "Z" {
				t.Fatalf("normal fallback macro = %+v recording=%v", state.macroSteps, state.macroRecording)
			}
			for _, want := range test.undo {
				if !doc.Editor.Undo() || doc.Editor.String() != want {
					t.Fatalf("fallback source Undo = %q; want %q", doc.Editor.String(), want)
				}
			}
			if doc.Editor.CanUndo() {
				t.Fatal("fallback added extra ordinary history")
			}
		})
	}
}

func TestPublicCompositionRectangleEmptyUpdateRetainsPendingResult(t *testing.T) {
	state, driver := startDriver(t)
	doc := beginTrackedCompositionRectangle(t, state, driver)
	input := documentCompositionInput(t, state, 0)
	state.startMacro()
	sendComposition(t, input, text.CompositionUpdate, "pre")
	assertLiveComposition(t, doc)
	next := sendComposition(t, input, text.CompositionUpdate, "")
	if next.Value != "ab\ncdQ" || next.Composing || state.provisional.doc != nil || state.rectangles[0].composing || len(state.macroSteps) != 0 {
		t.Errorf("empty update stranded rectangle composition: next=%+v rectangle=%+v frame=%+v", next, state.rectangles[0], state.provisional)
	}
	next = sendComposition(t, input, text.CompositionResult, "Z")
	if next.Value != "aZb\ncZdQ" || next.Composing || state.provisional.doc != nil || !state.rectangles[0].active || state.rectangles[0].composing {
		t.Fatalf("rectangle pending Result lost real history: next=%+v rectangle=%+v frame=%+v", next, state.rectangles[0], state.provisional)
	}
	if state.macroRecording || len(state.macroSteps) != 0 {
		t.Fatal("rectangle Result recorded an unsupported macro step")
	}
	for _, want := range []string{"ab\ncdQ", "ab\ncd"} {
		if !doc.Editor.Undo() || doc.Editor.String() != want {
			t.Fatalf("rectangle pending Result Undo = %q; want %q", doc.Editor.String(), want)
		}
	}
	if doc.Editor.CanUndo() {
		t.Fatal("rectangle empty update fabricated an undo unit")
	}
}
