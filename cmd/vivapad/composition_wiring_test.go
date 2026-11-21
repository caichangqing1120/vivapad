package main

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/macro"
)

func documentCompositionInput(t *testing.T, state *model, pane int) *app.TextInput {
	t.Helper()
	input := state.documentTextInput(pane)
	if input == nil || input.OnEvent == nil {
		t.Fatalf("documentTextInput(%d) = %+v", pane, input)
	}
	return input
}

func sendComposition(t *testing.T, input *app.TextInput, phase text.CompositionPhase, value string) app.TextState {
	t.Helper()
	kind := app.TextSetMarked
	switch phase {
	case text.CompositionResult:
		kind = app.TextInsert
	case text.CompositionCommit, text.CompositionCancel:
		kind = app.TextUnmark
	case text.CompositionEmptyCommit:
		kind = app.TextInsert
	}
	marked := app.TextRange{Start: len([]rune(value)), End: len([]rune(value))}
	state, err := input.OnEvent(app.TextEvent{
		Kind: kind, Text: value, MarkedSelection: marked, CompositionPhase: phase,
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// tentativeTextInput resolves a pane input after explicitly establishing the
// tentative capability on that pane editor. Non-Darwin builds never auto-enable
// it, so a fixture that needs a live tentative must opt in itself and assert the
// resolved input routes the capability; production platform gating is unchanged.
func tentativeTextInput(t *testing.T, state *model, pane int) *app.TextInput {
	t.Helper()
	doc := state.paneDocument(pane)
	if doc == nil || doc.Editor == nil {
		t.Fatalf("pane %d has no document editor", pane)
	}
	if !doc.Editor.TentativeCompositionEnabled() && !doc.Editor.EnableTentativeComposition() {
		t.Fatalf("pane %d document editor refused tentative composition", pane)
	}
	input := documentCompositionInput(t, state, pane)
	if !input.State.TentativeComposition {
		t.Fatalf("pane %d input did not route the tentative capability: %+v", pane, input.State)
	}
	return input
}

// assertLiveComposition fails a fixture that silently never established a live
// tentative composition.
func assertLiveComposition(t *testing.T, doc *document.Document) {
	t.Helper()
	if _, composing := doc.Editor.Composition(); !composing {
		t.Fatal("fixture did not establish a live tentative composition")
	}
}

func TestDocumentCompositionCapabilityIsLimitedToDarwinDocumentEditors(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	input := documentCompositionInput(t, state, 0)
	if runtime.GOOS != "darwin" {
		if doc.Editor.TentativeCompositionEnabled() || input.State.TentativeComposition {
			t.Fatal("non-Darwin document editor enabled tentative composition")
		}
		// The Driver and the assertions must resolve the one model this test
		// built; startDriver must never be called a second time here.
		assertLegacyDriverDocumentComposition(t, state, driver)
		return
	}
	if !doc.Editor.TentativeCompositionEnabled() || !input.State.TentativeComposition {
		t.Fatal("document editor did not opt into tentative composition")
	}
	if state.compositionOwner.editor != doc.Editor || state.compositionOwner.target != editorID {
		t.Fatalf("composition owner = %+v", state.compositionOwner)
	}
	for name, editor := range map[string]*text.Editor{
		"query": state.query, "replacement": state.replacement, "go-to": state.goToField,
		"files-root": state.filesRoot, "function-filter": state.functionFilter,
	} {
		if editor.TentativeCompositionEnabled() {
			t.Fatalf("%s editor enabled tentative composition", name)
		}
	}
}

// assertLegacyDriverDocumentComposition drives the capability-disabled legacy
// contract through the same model the Driver edits. The non-Darwin branch and
// the capability-fallback regression share it so neither can assert on a
// different Driver model than the one it drove.
func assertLegacyDriverDocumentComposition(t *testing.T, state *model, driver *app.Driver) {
	t.Helper()
	doc := state.space.Current()
	if doc.Editor.TentativeCompositionEnabled() {
		t.Fatal("legacy document editor retained tentative capability")
	}
	clickID(t, driver, editorID)
	if err := driver.Compose("x"); err != nil {
		t.Fatal(err)
	}
	if _, composing := doc.Editor.Composition(); !composing {
		t.Fatal("legacy headless composition did not mark the document editor the Driver edits")
	}
	if got := doc.Editor.String(); got != "x" {
		t.Fatalf("legacy composition reached another model: %q", got)
	}
	if state.space.Current() != doc {
		t.Fatal("Driver resolved a different current document")
	}
}

// TestCompositionLegacyNonDarwinUsesSameModel forces the non-Darwin legacy
// contract on every platform: a restricted editor refuses the tentative
// capability, so the document field keeps the legacy contract while the Driver,
// the resolved input and the assertions all use the model this test built.
func TestCompositionLegacyNonDarwinUsesSameModel(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Restrict(func(rune) bool { return true }, 1)
	input := documentCompositionInput(t, state, 0)
	if input.State.TentativeComposition || doc.Editor.TentativeCompositionEnabled() {
		t.Fatal("restricted editor advertised tentative composition")
	}
	// The fallback map is a Darwin-only app record: enableDocumentComposition
	// returns before recording it when the platform never opts in. The legacy
	// Driver contract below is asserted on every platform.
	if runtime.GOOS == "darwin" && !state.compositionFallback[doc.Editor] {
		t.Fatal("legacy fallback was not recorded on the driven model")
	}
	if state.compositionOwner.editor != doc.Editor || state.compositionOwner.target != editorID {
		t.Fatalf("legacy composition owner = %+v", state.compositionOwner)
	}
	assertLegacyDriverDocumentComposition(t, state, driver)
	if err := driver.Compose(""); err != nil {
		t.Fatal(err)
	}
	if _, composing := doc.Editor.Composition(); composing {
		t.Fatal("legacy unmark left marked text on the driven model")
	}
	if got := doc.Editor.String(); got != "x" {
		t.Fatalf("legacy unmark committed on another model: %q", got)
	}
}
func TestDocumentCompositionEnableFailureFallsBackOnce(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Restrict(func(rune) bool { return true }, 1)
	input := documentCompositionInput(t, state, 0)
	if doc.Editor.TentativeCompositionEnabled() || input.State.TentativeComposition {
		t.Fatal("restricted editor advertised tentative composition")
	}
	if runtime.GOOS == "darwin" {
		// The fallback map is the Darwin capability contract: on other
		// platforms enable is never even attempted, so nothing is recorded.
		if !state.compositionFallback[doc.Editor] {
			t.Fatal("enable failure was not retained as an explicit fallback")
		}
		for range 3 {
			again := documentCompositionInput(t, state, 0)
			if again.State.TentativeComposition || doc.Editor.TentativeCompositionEnabled() {
				t.Fatal("fallback retried capability or faked success")
			}
		}
		if len(state.compositionFallback) != 1 {
			t.Fatalf("fallback map = %+v", state.compositionFallback)
		}
		return
	}
	// Non-Darwin legacy contract: the same restricted editor never advertises
	// the capability and no fallback record is fabricated.
	for range 3 {
		again := documentCompositionInput(t, state, 0)
		if again.State.TentativeComposition || doc.Editor.TentativeCompositionEnabled() {
			t.Fatal("non-Darwin legacy input advertised tentative composition")
		}
	}
	if len(state.compositionFallback) != 0 {
		t.Fatalf("non-Darwin fallback map = %+v", state.compositionFallback)
	}
}
func TestReleaseDocumentCancelsDisablesAndResetsComposition(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("abc")
	doc.Editor.MoveTo(1, false)
	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "xy")
	assertLiveComposition(t, doc)
	if doc.Editor.String() != "axybc" {
		t.Fatalf("provisional update = %q", doc.Editor.String())
	}
	state.releaseDocument(doc)
	if doc.Editor.String() != "abc" || doc.Editor.TentativeCompositionEnabled() {
		t.Fatalf("released editor = %q capability=%v", doc.Editor.String(), doc.Editor.TentativeCompositionEnabled())
	}
	if _, composing := doc.Editor.Composition(); composing {
		t.Fatal("released editor retained marked text")
	}
	if state.provisional.doc != nil || state.compositionOwner.editor != nil || !state.compositionOwner.target.IsZero() {
		t.Fatalf("app ownership survived release: provisional=%+v owner=%+v", state.provisional, state.compositionOwner)
	}
	if len(state.compositionFallback) != 0 {
		t.Fatalf("released fallback map = %+v", state.compositionFallback)
	}
}

func TestReadOnlyTransitionCancelsBeforeResolvingInput(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("abc")
	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "x")
	assertLiveComposition(t, doc)
	state.readOnly[doc] = true
	if input := state.documentTextInput(0); input != nil {
		t.Fatalf("read-only input = %+v", input)
	}
	if doc.Editor.String() != "abc" {
		t.Fatalf("read-only rollback = %q", doc.Editor.String())
	}
	if _, composing := doc.Editor.Composition(); composing || state.provisional.doc != nil {
		t.Fatal("read-only transition stranded a live composition")
	}
	delete(state.readOnly, doc)
	documentCompositionInput(t, state, 0)
}

func TestCommandCancelsCompositionBeforeCapturingPreimage(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab")
	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "X")
	assertLiveComposition(t, doc)
	if doc.Editor.String() != "abX" {
		t.Fatalf("preimage composition = %q", doc.Editor.String())
	}
	state.command("Duplicate Line")
	if doc.Editor.String() != "ab\nab" {
		t.Fatalf("command used provisional preimage: %q", doc.Editor.String())
	}
	if state.provisional.doc != nil {
		t.Fatal("command retained provisional frame")
	}
}

func TestTextOwnerChangeCancelsPreviousComposition(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	state.splitEnabled = true
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.activePane = 0

	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "x")
	assertLiveComposition(t, left)
	if left.Editor.String() != "leftx" {
		t.Fatalf("left preedit = %q", left.Editor.String())
	}
	documentCompositionInput(t, state, 1)
	if left.Editor.String() != "left" {
		t.Fatalf("old target retained preedit: %q", left.Editor.String())
	}
	if state.compositionOwner.editor != right.Editor || state.compositionOwner.target != rightEditorID {
		t.Fatalf("new owner = %+v", state.compositionOwner)
	}
	if state.provisional.doc != nil {
		t.Fatalf("old provisional frame = %+v", state.provisional)
	}
}

func TestSharedCloneTargetChangeCancelsInsteadOfStrandingMark(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("shared")
	state.splitEnabled = true
	state.paneDocs[0], state.paneDocs[1] = doc, doc

	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "x")
	assertLiveComposition(t, doc)
	documentCompositionInput(t, state, 1)
	if doc.Editor.String() != "shared" {
		t.Fatalf("shared clone retained preedit: %q", doc.Editor.String())
	}
	if _, composing := doc.Editor.Composition(); composing {
		t.Fatal("shared clone stranded marked text")
	}
	if state.compositionOwner.target != rightEditorID {
		t.Fatalf("shared clone owner target = %v", state.compositionOwner.target)
	}
}

func TestNonEditorTargetAndPointerAwayCancelComposition(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("abc")
	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "x")
	assertLiveComposition(t, doc)
	state.reconcileTextOwner(queryID, &app.TextInput{Editor: state.query})
	if doc.Editor.String() != "abc" || state.compositionOwner.target != queryID {
		t.Fatalf("dialog target did not cancel: %q owner=%+v", doc.Editor.String(), state.compositionOwner)
	}
	state.reconcileTextOwner(gfx.ID{}, nil)
	if state.compositionOwner.editor != nil || !state.compositionOwner.target.IsZero() {
		t.Fatalf("nil target owner = %+v", state.compositionOwner)
	}

	input = documentCompositionInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "y")
	assertLiveComposition(t, doc)
	state.pointerCompositionAway(editorID)
	if doc.Editor.String() != "abcy" || state.provisional.doc != doc {
		t.Fatal("pointer on the owning editor canceled composition")
	}
	state.pointerCompositionAway(queryID)
	if doc.Editor.String() != "abc" || state.provisional.doc != nil {
		t.Fatalf("pointer away = %q provisional=%+v", doc.Editor.String(), state.provisional)
	}
}

func TestMacroRecordsOneCommittedCompositionStep(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	state.startMacro()
	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "n")
	assertLiveComposition(t, doc)
	sendComposition(t, input, text.CompositionUpdate, "ni")
	if len(state.macroSteps) != 0 {
		t.Fatalf("provisional steps = %+v", state.macroSteps)
	}
	sendComposition(t, input, text.CompositionResult, "你")
	if len(state.macroSteps) != 1 || state.macroSteps[0].Kind != macro.TextStep ||
		state.macroSteps[0].TextKind != app.TextInsert || state.macroSteps[0].Text != "你" {
		t.Fatalf("committed macro steps = %+v", state.macroSteps)
	}
	start := doc.Editor.String()
	sendComposition(t, input, text.CompositionUpdate, "m")
	sendComposition(t, input, text.CompositionCancel, "")
	if doc.Editor.String() != start || len(state.macroSteps) != 1 {
		t.Fatalf("cancel macro steps = %+v text=%q", state.macroSteps, doc.Editor.String())
	}
}

func TestProvisionalCompositionDefersBookmarkAnchorAndWordCompletion(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb")
	doc.Editor.MoveTo(0, false)
	state.setBookmarkLines(doc, []int{1})
	state.selectAnchors[0] = selectAnchor{doc: doc, index: 2, value: doc.Editor.String()}
	state.wordCompletion = &wordCompletionState{doc: doc, source: "b", start: 2, end: 2}

	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "\n")
	assertLiveComposition(t, doc)
	if stored := state.bookmarks[doc]; stored == nil || len(stored.lines) != 1 || stored.lines[0] != 1 {
		t.Fatalf("provisional bookmarks = %+v", stored)
	}
	if state.selectAnchors[0].index != 2 || state.wordCompletion == nil {
		t.Fatalf("provisional trackers moved: anchor=%+v completion=%+v", state.selectAnchors[0], state.wordCompletion)
	}
	sendComposition(t, input, text.CompositionCommit, "")
	if got := state.bookmarkLines(doc); len(got) != 1 || got[0] != 2 {
		t.Fatalf("committed bookmarks = %v", got)
	}
	if state.selectAnchors[0].index != 3 {
		t.Fatalf("committed select anchor = %+v", state.selectAnchors[0])
	}
	if state.provisional.doc != nil {
		t.Fatalf("committed provisional frame = %+v", state.provisional)
	}
}

func TestPaintDoesNotAdvanceProvisionalBookmarksOrWordCompletion(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("a\nb")
	doc.Editor.MoveTo(0, false)
	state.setBookmarkLines(doc, []int{1})
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	state.wordCompletion = &wordCompletionState{doc: doc, source: doc.Editor.String(), start: 0, end: 0}
	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "\n")
	assertLiveComposition(t, doc)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if stored := state.bookmarks[doc]; stored == nil || len(stored.lines) != 1 || stored.lines[0] != 1 {
		t.Fatalf("paint advanced provisional bookmarks: %+v", stored)
	}
	if state.wordCompletion == nil {
		t.Fatal("paint discarded provisional word completion")
	}
	sendComposition(t, input, text.CompositionCommit, "")
	if got := state.bookmarkLines(doc); len(got) != 1 || got[0] != 2 {
		t.Fatalf("committed bookmarks = %v", got)
	}
}

func TestExplicitResultUsesRawUTF16Provenance(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("A𝄞BC")
	doc.Editor.MoveTo(0, false)
	input := tentativeTextInput(t, state, 0)
	sendComposition(t, input, text.CompositionUpdate, "x")
	assertLiveComposition(t, doc)
	next, err := input.OnEvent(app.TextEvent{
		Kind: app.TextInsert, CompositionPhase: text.CompositionResult, Text: "Z",
		Replacement: app.TextRange{Start: 3, End: 4}, HasReplacement: true,
		ReplacementUTF16: text.UTF16Range{Start: 3, Length: 1}, HasReplacementUTF16: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "A𝄞ZC" {
		t.Fatalf("raw UTF-16 result = %q", doc.Editor.String())
	}
	if next.Composing || state.provisional.doc != nil {
		t.Fatalf("raw result left composition: %+v provisional=%+v", next, state.provisional)
	}
}

func TestTaggedEventsWithoutCapabilityDoNotTakeTentativePath(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("abc")
	doc.Editor.MoveTo(1, false)
	state.setBookmarkLines(doc, []int{0})
	state.selectAnchors[0] = selectAnchor{doc: doc, index: 1, value: doc.Editor.String()}
	state.wordCompletion = &wordCompletionState{doc: doc, source: "abc", start: 1, end: 1}
	state.startMacro()
	state.multiSelections[0] = multiSelectionState{
		doc: doc, value: "abc", active: true, ranges: []text.Range{{Start: 0, End: 0}},
	}
	state.rectangles[0] = rectangleSelection{
		doc: doc, anchor: 1, caret: 1, value: "abc", active: true,
	}
	input := documentCompositionInput(t, state, 0)
	doc.Editor.Restrict(func(rune) bool { return true }, 1)
	if doc.Editor.TentativeCompositionEnabled() {
		t.Fatal("restricted editor unexpectedly retained tentative capability")
	}
	before := captureComposition(state, doc)
	fallbackEvent := app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "x"}
	fallbackEvent.Apply(doc.Editor)
	if doc.Editor.String() != "abc" {
		t.Fatalf("direct fallback event changed disabled editor: %q", doc.Editor.String())
	}
	next, err := input.OnEvent(app.TextEvent{
		Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "abc" || next.Composing {
		t.Fatalf("tagged event without capability became tentative: %q state=%+v", doc.Editor.String(), next)
	}
	if _, composing := doc.Editor.Composition(); composing {
		t.Fatal("tagged event created legacy composition")
	}
	after := captureComposition(state, doc)
	if after.value != "abc" || after.capability {
		t.Fatalf("fallback event snapshot = %+v", after)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("fallback event changed app state\nbefore=%+v\nafter=%+v", before, after)
	}
}
