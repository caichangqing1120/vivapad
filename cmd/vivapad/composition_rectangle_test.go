package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func beginCompositionRectangle(t *testing.T, state *model, driver *app.Driver) *document.Document {
	t.Helper()
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	// Establish the tentative capability explicitly. On non-Darwin the app never
	// auto-enables it, and these fixtures must exercise the tagged path there
	// too; production platform gating stays unchanged.
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("document editor refused tentative composition")
	}
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	return doc
}

// beginTrackedCompositionRectangle builds the same rectangle frame but keeps
// one known pre-composition undo unit ("ab\ncd" + the Q edit) and a non-empty
// bookmark/Find Mark/user-style/word-completion frame, so a rejected terminal
// can be proven not to damage real tracker state or history.
func beginTrackedCompositionRectangle(t *testing.T, state *model, driver *app.Driver) *document.Document {
	t.Helper()
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	doc.Editor.Insert("Q")
	if got := doc.Editor.String(); got != "ab\ncdQ" || !doc.Editor.CanUndo() {
		t.Fatalf("tracked fixture history = %q canUndo=%v", got, doc.Editor.CanUndo())
	}
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("document editor refused tentative composition")
	}
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	return doc
}

func beginTaggedCompositionRectangle(t *testing.T, state *model, driver *app.Driver) (*document.Document, *app.TextInput) {
	t.Helper()
	doc := beginCompositionRectangle(t, state, driver)
	return doc, documentCompositionInput(t, state, 0)
}

func TestRectangleCompositionCommitUsesEveryRowAsOneUndo(t *testing.T) {
	state, driver := startDriver(t)
	doc := beginCompositionRectangle(t, state, driver)
	input := documentCompositionInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "zhong")
	assertLiveComposition(t, doc)
	if doc.Editor.String() != "ab\nczhongd" || !state.rectangles[0].active {
		t.Fatalf("rectangle preedit changed rows: %q rect=%+v", doc.Editor.String(), state.rectangles[0])
	}
	if state.provisional.doc != doc || !state.rectangles[0].composing {
		t.Fatalf("rectangle provisional = %+v rect=%+v", state.provisional, state.rectangles[0])
	}
	sendComposition(t, input, text.CompositionCommit, "")
	if doc.Editor.String() != "azhongb\nczhongd" || !state.rectangles[0].active || state.provisional.doc != nil {
		t.Fatalf("rectangle commit = %q rect=%+v provisional=%+v", doc.Editor.String(), state.rectangles[0], state.provisional)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd" || doc.Editor.CanUndo() {
		t.Fatalf("rectangle commit undo = %q", doc.Editor.String())
	}
}

func TestRectangleCompositionResultUsesEventTextOnEveryRow(t *testing.T) {
	state, driver := startDriver(t)
	doc := beginCompositionRectangle(t, state, driver)
	input := documentCompositionInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "zh")
	assertLiveComposition(t, doc)
	sendComposition(t, input, text.CompositionResult, "中")
	if doc.Editor.String() != "a中b\nc中d" || state.provisional.doc != nil {
		t.Fatalf("rectangle result = %q provisional=%+v", doc.Editor.String(), state.provisional)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd" || doc.Editor.CanUndo() {
		t.Fatalf("rectangle result undo = %q", doc.Editor.String())
	}
}

func TestRectangleCompositionCancelRollsBackWithoutTouchingRows(t *testing.T) {
	state, driver := startDriver(t)
	doc := beginCompositionRectangle(t, state, driver)
	input := documentCompositionInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "zhong")
	assertLiveComposition(t, doc)
	sendComposition(t, input, text.CompositionCancel, "")
	if doc.Editor.String() != "ab\ncd" || !state.rectangles[0].active || state.rectangles[0].composing {
		t.Fatalf("rectangle cancel = %q rect=%+v", doc.Editor.String(), state.rectangles[0])
	}
	if doc.Editor.CanUndo() {
		t.Fatal("rectangle cancel left undo history")
	}
	if state.provisional.doc != nil {
		t.Fatalf("rectangle cancel provisional = %+v", state.provisional)
	}
}

func TestMultiSelectionCompositionFallsBackWithoutDuplicatingPreedit(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("a a")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	state.multiSelectAll(document.SearchOptions{})
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	// Non-Darwin builds never auto-enable the capability; the multi path needs a
	// live tentative to consume the tagged update.
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("document editor refused tentative composition")
	}
	input := documentCompositionInput(t, state, 0)
	if !input.State.TentativeComposition {
		t.Fatalf("multi input did not route the tentative capability: %+v", input.State)
	}
	sendComposition(t, input, text.CompositionUpdate, "x")
	assertLiveComposition(t, doc)
	if state.multiSelections[0].active || strings.Count(doc.Editor.String(), "x") != 1 {
		t.Fatalf("multi composition duplicated: %q multi=%+v", doc.Editor.String(), state.multiSelections[0])
	}
	sendComposition(t, input, text.CompositionCommit, "")
	if doc.Editor.String() != "a x" || strings.Count(doc.Editor.String(), "x") != 1 {
		t.Fatalf("multi composition commit = %q", doc.Editor.String())
	}
}

func TestInvalidMultiSelectionCompositionKeepsRanges(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectAll(document.SearchOptions{})
	before := append([]text.Range(nil), state.multiSelections[0].ranges...)
	// Establish the capability so the invalid phase reaches the composition
	// handler instead of short-circuiting on a disabled editor.
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("document editor refused tentative composition")
	}
	input := state.documentTextInput(0)
	if input == nil || !input.State.TentativeComposition {
		t.Fatalf("invalid multi input = %+v", input)
	}
	_, _ = input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionPhase(202), Text: "bad"})
	if !state.multiSelections[0].active || len(state.multiSelections[0].ranges) != len(before) {
		t.Fatalf("invalid composition cleared multi: %+v", state.multiSelections[0])
	}
}

func TestReadOnlyCompositionRejectsRectangleAndMultiText(t *testing.T) {
	state, driver := startDriver(t)
	doc := beginCompositionRectangle(t, state, driver)
	input := documentCompositionInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "zh")
	assertLiveComposition(t, doc)
	state.readOnly[doc] = true
	if input := state.documentTextInput(0); input != nil {
		t.Fatal("read-only rectangle resolved an input")
	}
	if doc.Editor.String() != "ab\ncd" || state.provisional.doc != nil {
		t.Fatalf("read-only rectangle = %q provisional=%+v", doc.Editor.String(), state.provisional)
	}
}

func TestRectangleRejectedResultPreservesLiveComposition(t *testing.T) {
	cases := []struct {
		name  string
		event app.TextEvent
	}{
		{
			name:  "malformed UTF-8 result",
			event: app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: string([]byte{0xff, 0xfe})},
		},
		{
			name: "negative raw UTF-16 replacement",
			event: app.TextEvent{
				Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "x",
				HasReplacementUTF16: true, ReplacementUTF16: text.UTF16Range{Start: -1, Length: 1},
			},
		},
		{
			name: "wrong replacement range",
			event: app.TextEvent{
				Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "x",
				HasReplacement: true, Replacement: app.TextRange{Start: -1, End: 2},
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := beginTrackedCompositionRectangle(t, state, driver)
			// Non-empty tracker frame: bookmark line, Find Mark span over "cd",
			// a user-style span over "ab", word completion and macro recording.
			state.setBookmarkLines(doc, []int{1})
			state.markedDocument, state.markedValue = doc, doc.Editor.String()
			state.markedSpans = []text.Span{{Start: 3, End: 5}}
			state.markedTracking = trackIndicators(doc.Editor)
			state.styledMarks = map[*document.Document]*styleMarkState{
				doc: {tracking: trackIndicators(doc.Editor), styles: [5][]text.Span{{{Start: 0, End: 2}}}},
			}
			state.wordCompletion = &wordCompletionState{
				kind: completionWord, doc: doc, pane: 0, base: doc.Editor.String(), source: "ab",
				start: 0, end: 2, items: []string{"abacus", "abandon"}, selected: 1, offset: 3,
			}
			state.startMacro()

			input := documentCompositionInput(t, state, 0)
			if !doc.Editor.TentativeCompositionEnabled() || input.State.TentativeComposition != true {
				t.Fatalf("tracked fixture lacks tentative capability: editor=%v input=%+v", doc.Editor.TentativeCompositionEnabled(), input.State)
			}
			sendComposition(t, input, text.CompositionUpdate, "zhong")
			if !state.rectangles[0].composing || state.provisional.doc != doc {
				t.Fatalf("live rectangle composition = %+v provisional=%+v", state.rectangles[0], state.provisional)
			}
			if doc.Editor.String() != "ab\nczhongdQ" {
				t.Fatalf("tracked fixture preedit = %q", doc.Editor.String())
			}
			before := captureComposition(state, doc)
			beforeRectangle := state.rectangles[0]
			beforeStyles := append([]text.Span(nil), state.styledMarks[doc].styles[0]...)
			beforeCompletion := snapshotWordCompletion(state.wordCompletion)

			next, err := input.OnEvent(test.event)
			if err != nil {
				t.Fatal(err)
			}
			after := captureComposition(state, doc)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("rejected result changed state\nbefore=%+v\nafter=%+v", before, after)
			}
			if state.rectangles[0] != beforeRectangle {
				t.Fatalf("rejected result moved rectangle\nbefore=%+v\nafter=%+v", beforeRectangle, state.rectangles[0])
			}
			if got := state.styledMarks[doc]; got == nil || !reflect.DeepEqual(got.styles[0], beforeStyles) {
				t.Fatalf("rejected result moved user styles: %+v", got)
			}
			if got := state.wordCompletion; got == nil || got.doc != doc || got.kind != completionWord ||
				!reflect.DeepEqual(got.items, []string{"abacus", "abandon"}) || got.selected != 1 || got.offset != 3 {
				t.Fatalf("rejected result changed the completion candidate list: %+v", got)
			}
			if next != app.TextStateOf(doc.Editor) {
				t.Fatalf("returned state = %+v want %+v", next, app.TextStateOf(doc.Editor))
			}
			if !after.composing || after.provisional.doc != doc || !state.rectangles[0].composing {
				t.Fatalf("rejection lost live composition: composing=%v provisional=%+v rect=%+v", after.composing, after.provisional, state.rectangles[0])
			}
			if !after.canUndo || after.canRedo || len(state.macroSteps) != 0 {
				t.Fatalf("rejected result damaged history/macro: canUndo=%v canRedo=%v macro=%+v", after.canUndo, after.canRedo, state.macroSteps)
			}
			if doc.Editor.Undo() {
				t.Fatal("undo ran during a live tentative")
			}
			if doc.Editor.String() != before.value {
				t.Fatalf("refused undo changed the live preedit: %q", doc.Editor.String())
			}

			// The live composition is still cancelable exactly once; the cancel
			// leaves every tracker at its pre-composition frame.
			next = sendComposition(t, input, text.CompositionCancel, "")
			if doc.Editor.String() != "ab\ncdQ" || next.Composing || state.provisional.doc != nil ||
				state.rectangles[0].composing {
				t.Fatalf("cancel after rejection = %q state=%+v provisional=%+v rect=%+v", doc.Editor.String(), next, state.provisional, state.rectangles[0])
			}
			canceled := captureComposition(state, doc)
			if !reflect.DeepEqual(canceled.bookmark, []int{1}) ||
				!reflect.DeepEqual(canceled.marked, before.marked) ||
				!reflect.DeepEqual(canceled.word, before.word) ||
				!reflect.DeepEqual(canceled.word, beforeCompletion) ||
				!reflect.DeepEqual(state.styledMarks[doc].styles[0], beforeStyles) {
				t.Fatalf("cancel lost tracker frame: bookmark=%v marked=%v word=%+v styles=%+v",
					canceled.bookmark, canceled.marked, canceled.word, state.styledMarks[doc])
			}
			if got := state.wordCompletion; got == nil || !reflect.DeepEqual(got.items, []string{"abacus", "abandon"}) ||
				got.selected != 1 || got.offset != 3 {
				t.Fatalf("cancel changed the completion candidate list: %+v", got)
			}

			// Replay the pre-composition history: the Rejected Result + Cancel
			// must not fabricate or destroy undo units.
			if !doc.Editor.Undo() {
				t.Fatal("canceled composition destroyed the pre-composition undo history")
			}
			if got := doc.Editor.String(); got != "ab\ncd" {
				t.Fatalf("history replay = %q", got)
			}
			if doc.Editor.CanUndo() || doc.Editor.Undo() {
				t.Fatal("cancel fabricated extra undo history")
			}
		})
	}
}
