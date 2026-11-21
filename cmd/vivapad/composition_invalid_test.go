package main

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

// wordCompletionSnapshot is an independent copy of wordCompletionState. It
// stores no pointer to the live struct, so a paint that mutates the completion
// in place is visible to the comparison instead of silently mutating the
// captured value too.
type wordCompletionSnapshot struct {
	kind     completionKind
	doc      *document.Document
	pane     int
	base     string
	source   string
	start    int
	end      int
	items    []string
	selected int
	offset   int
}

func snapshotWordCompletion(current *wordCompletionState) *wordCompletionSnapshot {
	if current == nil {
		return nil
	}
	return &wordCompletionSnapshot{
		kind: current.kind, doc: current.doc, pane: current.pane, base: current.base,
		source: current.source, start: current.start, end: current.end,
		items: append([]string(nil), current.items...), selected: current.selected, offset: current.offset,
	}
}

type compositionSnapshot struct {
	value        string
	selection    text.Range
	caret        int
	composition  text.Range
	composing    bool
	capability   bool
	canUndo      bool
	canRedo      bool
	provisional  provisionalComposition
	bookmark     []int
	bookmarkBase string
	word         *wordCompletionSnapshot
	macroSteps   int
	marked       []text.Span
}

func captureComposition(state *model, doc *document.Document) compositionSnapshot {
	snapshot := compositionSnapshot{
		value:       doc.Editor.String(),
		selection:   doc.Editor.Selection(),
		caret:       doc.Editor.Caret(),
		capability:  doc.Editor.TentativeCompositionEnabled(),
		canUndo:     doc.Editor.CanUndo(),
		canRedo:     doc.Editor.CanRedo(),
		provisional: state.provisional,
		word:        snapshotWordCompletion(state.wordCompletion),
		macroSteps:  len(state.macroSteps),
		marked:      append([]text.Span(nil), state.markedSpans...),
	}
	snapshot.composition, snapshot.composing = doc.Editor.Composition()
	if stored := state.bookmarks[doc]; stored != nil {
		snapshot.bookmark = append([]int(nil), stored.lines...)
		snapshot.bookmarkBase = stored.value
	}
	return snapshot
}

func liveCompositionState(t *testing.T) (*model, *document.Document, *app.TextInput) {
	t.Helper()
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("seed")
	state.setBookmarkLines(doc, []int{0})
	state.wordCompletion = &wordCompletionState{
		kind: completionWord, doc: doc, pane: 0, base: "seed", source: "seed",
		start: 0, end: 4, items: []string{"seeds", "seedling"}, selected: 1, offset: 0,
	}
	state.markedDocument, state.markedValue = doc, doc.Editor.String()
	state.markedSpans = []text.Span{{Start: 0, End: 4}}
	state.startMacro()
	// Establish the tentative capability explicitly: non-Darwin builds never
	// auto-enable it, and every test using this fixture needs a live tentative.
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("document editor refused tentative composition")
	}
	input := documentCompositionInput(t, state, 0)
	if !input.State.TentativeComposition {
		t.Fatalf("input did not route the tentative capability: %+v", input.State)
	}
	sendComposition(t, input, text.CompositionUpdate, "ok")
	if _, composing := doc.Editor.Composition(); !composing {
		t.Fatal("fixture did not establish a live tentative composition")
	}
	return state, doc, input
}

func TestInvalidLiveCompositionEventsDoNotMutateState(t *testing.T) {
	cases := []struct {
		name  string
		event app.TextEvent
	}{
		{name: "unknown phase", event: app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionPhase(200), Text: "bad"}},
		{name: "invalid utf8 update", event: app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: string([]byte{0xff, 0xfe})}},
		{name: "replacement during update", event: app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "bad", Replacement: app.TextRange{Start: 0, End: 1}, HasReplacement: true}},
		{name: "negative raw replacement", event: app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "bad", ReplacementUTF16: text.UTF16Range{Start: -1, Length: 1}, HasReplacementUTF16: true}},
		{name: "invalid utf8 terminal", event: app.TextEvent{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: string([]byte{0x80})}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			state, doc, input := liveCompositionState(t)
			before := captureComposition(state, doc)
			next, err := input.OnEvent(test.event)
			if err != nil {
				t.Fatal(err)
			}
			after := captureComposition(state, doc)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("state changed\nbefore=%+v\nafter=%+v", before, after)
			}
			if next != app.TextStateOf(doc.Editor) {
				t.Fatalf("returned state = %+v want %+v", next, app.TextStateOf(doc.Editor))
			}
			if !after.composing || after.provisional.doc != doc {
				t.Fatalf("terminal rejection lost live composition: composing=%v provisional=%+v", after.composing, after.provisional)
			}
		})
	}
}

func TestStaleTerminalEventsAfterOwnerReleaseDoNotMutateState(t *testing.T) {
	state, doc, input := liveCompositionState(t)
	sendComposition(t, input, text.CompositionCancel, "")
	state.releaseDocumentComposition(doc)
	if doc.Editor.TentativeCompositionEnabled() {
		t.Fatal("retired callback still owns a composition capability")
	}
	before := captureComposition(state, doc)
	for _, event := range []app.TextEvent{
		{Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "stale"},
		{Kind: app.TextUnmark, CompositionPhase: text.CompositionCommit},
		{Kind: app.TextUnmark, CompositionPhase: text.CompositionCancel},
	} {
		if _, err := input.OnEvent(event); err != nil {
			t.Fatal(err)
		}
		after := captureComposition(state, doc)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("stale event %+v changed state\nbefore=%+v\nafter=%+v", event, before, after)
		}
	}
}

func TestTerminalRejectionKeepsProvisionalFrameAndCancelStillWorks(t *testing.T) {
	state, doc, input := liveCompositionState(t)
	before := captureComposition(state, doc)
	sendComposition(t, input, text.CompositionResult, string([]byte{0xff}))
	after := captureComposition(state, doc)
	if !reflect.DeepEqual(before, after) || !after.composing || after.provisional.doc != doc {
		t.Fatalf("rejected terminal changed frame: before=%+v after=%+v", before, after)
	}
	next := sendComposition(t, input, text.CompositionCancel, "")
	if doc.Editor.String() != "seed" || next.Composing || state.provisional.doc != nil {
		t.Fatalf("cancel after rejection = %q state=%+v provisional=%+v", doc.Editor.String(), next, state.provisional)
	}
}

func TestInvalidCompositionEventsRaceAcrossIndependentModels(t *testing.T) {
	const workers = 8
	var group sync.WaitGroup
	failures := make(chan string, workers)
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for iteration := 0; iteration < 20; iteration++ {
				state := newModel()
				doc := state.space.Current()
				doc.Editor.Insert("independent")
				state.setBookmarkLines(doc, []int{0})
				state.startMacro()
				if !doc.Editor.EnableTentativeComposition() {
					failures <- fmt.Sprintf("worker %d: editor refused tentative composition", worker)
					return
				}
				input := state.documentTextInput(0)
				if input == nil {
					failures <- fmt.Sprintf("worker %d: no input", worker)
					return
				}
				if !input.State.TentativeComposition {
					failures <- fmt.Sprintf("worker %d: input did not route the capability", worker)
					return
				}
				if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "ok"}); err != nil {
					failures <- fmt.Sprintf("worker %d: %v", worker, err)
					return
				}
				if _, composing := doc.Editor.Composition(); !composing {
					failures <- fmt.Sprintf("worker %d: no live tentative after update", worker)
					return
				}
				before := captureComposition(state, doc)
				for _, phase := range []text.CompositionPhase{text.CompositionPhase(201), text.CompositionUpdate, text.CompositionResult} {
					value := "bad"
					if phase != text.CompositionPhase(201) {
						value = string([]byte{0xff})
					}
					_, _ = input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: phase, Text: value})
					if after := captureComposition(state, doc); !reflect.DeepEqual(before, after) {
						failures <- fmt.Sprintf("worker %d phase %v changed %+v to %+v", worker, phase, before, after)
						return
					}
				}
				if !doc.Editor.TentativeCompositionEnabled() {
					failures <- fmt.Sprintf("worker %d: capability changed", worker)
					return
				}
			}
		}(worker)
	}
	group.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}
