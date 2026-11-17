package main

import (
	"runtime"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/macro"
)

// compositionOwner is the single app-owned native editor target. It is tracked
// separately from the editor's own capability so focus changes can cancel the
// previous live tentative before a new surface consumes events.
type compositionOwner struct {
	editor *text.Editor
	target gfx.ID
}

// provisionalComposition captures the preedit's rollback base and the selection
// anchor that existed before the first update. Bookmark and select-anchor
// tracking is applied once, from this frame, when the composition terminates.
type provisionalComposition struct {
	doc           *document.Document
	base          string
	ordinaryBase  string
	anchor        int
	ordinaryStart int
}

// documentTextInput resolves one of VivaPad's two document editor surfaces. It
// opts the editor into tentative composition only on Darwin and only after the
// capability succeeds; every other field remains legacy.
func (state *model) documentTextInput(pane int) *app.TextInput {
	if pane < 0 || pane > 1 {
		return nil
	}
	doc := state.paneDocument(pane)
	if doc == nil || doc.Editor == nil {
		state.cancelCompositionOwner()
		return nil
	}
	target := editorID
	if pane == 1 {
		target = rightEditorID
	}
	if state.isReadOnly(doc) {
		state.cancelDocumentComposition(doc)
		state.reconcileCompositionOwner(target, nil)
		return nil
	}
	state.reconcileCompositionOwner(target, doc.Editor)
	enabled := state.enableDocumentComposition(doc)
	var input *app.TextInput
	switch {
	case state.rectangles[pane].active && state.rectangles[pane].doc == doc:
		input = state.rectangleTextInput(pane)
	case state.multiSelections[pane].active && state.multiSelections[pane].doc == doc:
		input = state.multiSelectionTextInput(pane)
	default:
		input = state.macroTextInput(doc)
	}
	if input == nil {
		return nil
	}
	if !enabled {
		input.State = app.TextStateOf(doc.Editor)
	}
	return input
}

// resolveTextInput is the provider body for every native text field. The
// wrapper in newConfig reconciles ownership after it so a dialog or non-editor
// target cancels a live document composition exactly once.
func (state *model) resolveTextInput(context app.TextInputContext) *app.TextInput {
	if state.saveSessionOptionsOpen {
		return nil
	}
	if state.byteRangeOpen {
		if state.byteRangeMode == 2 && context.ID == byteRangeStartID {
			return &app.TextInput{Editor: state.byteRangeStart}
		}
		if state.byteRangeMode == 2 && context.ID == byteRangeEndID {
			return &app.TextInput{Editor: state.byteRangeEnd}
		}
	}
	if state.columnDialogOpen {
		switch context.ID {
		case columnTextID:
			return &app.TextInput{Editor: state.columnText}
		case columnInitialID:
			return &app.TextInput{Editor: state.columnInitial}
		case columnIncrementID:
			return &app.TextInput{Editor: state.columnIncrement}
		case columnRepeatID:
			return &app.TextInput{Editor: state.columnRepeat}
		}
		return nil
	}
	if state.preferencesOpen {
		return state.printPreferenceTextInput(context.ID)
	}
	if state.runDialogOpen {
		if !state.runSaveMode && context.ID == runDialogInputID {
			return &app.TextInput{Editor: state.runInput}
		}
		if state.runSaveMode && context.ID == runDialogNameID {
			return &app.TextInput{Editor: state.runName}
		}
		return nil
	}
	if state.summaryOpen {
		return nil
	}
	if state.windowsDialogOpen {
		return nil
	}
	if state.macroSaveOpen || state.macroRunOpen {
		if state.macroSaveOpen && context.ID == macroNameID {
			return &app.TextInput{Editor: state.macroName}
		}
		if state.macroRunOpen && context.ID == macroCountID {
			return &app.TextInput{Editor: state.macroCount}
		}
		return nil
	}
	if state.hashDialogOpen {
		if state.hashMode == hashFromText && context.ID == hashInputID {
			return &app.TextInput{Editor: state.hashInput}
		}
		return nil
	}
	if state.dateTimeDialogOpen {
		if context.ID == dateTimeFieldID {
			return &app.TextInput{Editor: state.dateTimeDraft}
		}
		return nil
	}
	if state.renameDoc != nil {
		if context.ID == renameFieldID {
			return &app.TextInput{Editor: state.renameField}
		}
		return nil
	}
	if state.renamePlan != nil || state.pendingTrash != nil {
		return nil
	}
	if state.pendingClose >= 0 || state.pendingReload >= 0 || state.replaceConfirmOpen {
		return nil
	}
	if state.filesDialogOpen && context.ID != queryID && context.ID != filesRootID && context.ID != filesPatternID {
		if !state.filesReplaceMode || context.ID != replacementID {
			return nil
		}
	}
	if state.functionListVisible && state.activeDock() == "functions" && context.ID == functionFilterID {
		return &app.TextInput{Editor: state.functionFilter}
	}
	switch context.ID {
	case editorID:
		return state.documentTextInput(0)
	case rightEditorID:
		return state.documentTextInput(1)
	case queryID:
		return &app.TextInput{Editor: state.query}
	case incrementQueryID:
		return state.incrementalTextInput()
	case replacementID:
		return &app.TextInput{Editor: state.replacement}
	case goToFieldID:
		return &app.TextInput{Editor: state.goToField}
	case filesRootID:
		return &app.TextInput{Editor: state.filesRoot}
	case filesPatternID:
		return &app.TextInput{Editor: state.filesPattern}
	}
	return nil
}

// reconcileTextOwner tracks (editor,target). A document target resolves its
// pane editor; any other target uses the input's editor or cancels ownership.
func (state *model) reconcileTextOwner(target gfx.ID, input *app.TextInput) {
	if input == nil {
		state.reconcileCompositionOwner(target, nil)
		return
	}
	editor := input.Editor
	if target == editorID || target == rightEditorID {
		if pane := paneForTarget(target); pane >= 0 {
			if doc := state.paneDocument(pane); doc != nil {
				editor = doc.Editor
			}
		}
	}
	state.reconcileCompositionOwner(target, editor)
}

func (state *model) reconcileCompositionOwner(target gfx.ID, editor *text.Editor) {
	if state.compositionOwner.editor == editor && state.compositionOwner.target == target {
		return
	}
	state.cancelCompositionOwner()
	state.compositionOwner = compositionOwner{editor: editor, target: target}
}

func (state *model) cancelCompositionOwner() {
	owner := state.compositionOwner
	state.cancelLiveComposition()
	state.provisional = provisionalComposition{}
	state.compositionOwner = compositionOwner{}
	if owner.editor != nil || owner.target != (gfx.ID{}) {
		state.compositionGeneration++
	}
}

func (state *model) cancelLiveComposition() {
	if state.provisional.doc != nil {
		state.cancelDocumentComposition(state.provisional.doc)
		return
	}
	if state.compositionOwner.editor != nil {
		state.cancelEditorComposition(state.compositionOwner.editor)
	}
}

// pointerCompositionAway cancels a live tentative when a pointer activates a
// different target. A click inside the owning editor keeps the composition.
func (state *model) pointerCompositionAway(target gfx.ID) {
	if state.compositionOwner.editor == nil || state.compositionOwner.target == target {
		return
	}
	state.cancelCompositionOwner()
}

func (state *model) enableDocumentComposition(doc *document.Document) bool {
	if runtime.GOOS != "darwin" || doc == nil || doc.Editor == nil {
		return false
	}
	if doc.Editor.TentativeCompositionEnabled() {
		return true
	}
	if state.compositionFallback == nil {
		state.compositionFallback = make(map[*text.Editor]bool)
	}
	if state.compositionFallback[doc.Editor] {
		return false
	}
	if !doc.Editor.EnableTentativeComposition() {
		state.compositionFallback[doc.Editor] = true
		if state.notice == "" {
			state.notice = "Input method composition is unavailable for this document; using legacy input"
		}
		return false
	}
	return true
}

func (state *model) cancelDocumentComposition(doc *document.Document) bool {
	if doc == nil || doc.Editor == nil {
		return false
	}
	return state.cancelEditorComposition(doc.Editor)
}

func (state *model) cancelEditorComposition(editor *text.Editor) bool {
	if editor == nil || !editor.TentativeCompositionEnabled() {
		if editor != nil && state.provisional.doc != nil && state.provisional.doc.Editor == editor {
			state.provisional = provisionalComposition{}
		}
		return false
	}
	if _, composing := editor.Composition(); !composing {
		if state.provisional.doc != nil && state.provisional.doc.Editor == editor {
			state.provisional = provisionalComposition{}
		}
		return false
	}
	if !editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel}) {
		return false
	}
	if state.provisional.doc != nil && state.provisional.doc.Editor == editor {
		state.provisional = provisionalComposition{}
	}
	return true
}

func (state *model) releaseDocumentComposition(doc *document.Document) {
	if doc == nil || doc.Editor == nil {
		return
	}
	state.cancelDocumentComposition(doc)
	if _, composing := doc.Editor.Composition(); composing {
		// A cancel refused by a stale boundary leaves the mark for the retired
		// editor; do not commit it or fabricate a successful capability.
		doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel})
	}
	doc.Editor.DisableTentativeComposition()
	if state.provisional.doc == doc {
		state.provisional = provisionalComposition{}
	}
	if state.compositionOwner.editor == doc.Editor {
		state.compositionOwner = compositionOwner{}
	}
	if state.compositionFallback != nil {
		delete(state.compositionFallback, doc.Editor)
	}
}

// applyDocumentComposition applies one tagged event. A false return is a
// rejected event: the editor and the app's provisional frame are untouched.
func (state *model) applyDocumentComposition(doc *document.Document, event app.TextEvent) app.TextState {
	next, _ := state.applyDocumentCompositionEvent(doc, event)
	return next
}

func (state *model) applyDocumentCompositionEvent(doc *document.Document, event app.TextEvent) (app.TextState, bool) {
	if doc == nil || doc.Editor == nil {
		return app.TextState{}, false
	}
	if event.CompositionPhase == text.CompositionNone || !doc.Editor.TentativeCompositionEnabled() {
		return app.TextStateOf(doc.Editor), false
	}
	_, composing := doc.Editor.Composition()
	switch event.CompositionPhase {
	case text.CompositionUpdate, text.CompositionResult:
	case text.CompositionCommit, text.CompositionCancel:
		if !composing {
			return app.TextStateOf(doc.Editor), false
		}
	case text.CompositionEmptyCommit:
		if !composing && !event.HasReplacement && !event.HasReplacementUTF16 {
			return app.TextStateOf(doc.Editor), false
		}
	default:
		return app.TextStateOf(doc.Editor), false
	}

	before := app.TextStateOf(doc.Editor)
	base := doc.Editor.String()
	anchor := doc.Editor.Selection().Start
	handled := doc.Editor.ApplyComposition(text.CompositionEvent{
		Phase:               event.CompositionPhase,
		Text:                event.Text,
		Selected:            text.Range{Start: event.MarkedSelection.Start, End: event.MarkedSelection.End},
		Replacement:         text.Range{Start: event.Replacement.Start, End: event.Replacement.End},
		HasReplacement:      event.HasReplacement,
		ReplacementUTF16:    event.ReplacementUTF16,
		HasReplacementUTF16: event.HasReplacementUTF16,
	})
	if !handled {
		return before, false
	}
	if event.CompositionPhase == text.CompositionUpdate {
		if marked, active := doc.Editor.Composition(); active {
			if state.provisional.doc != doc {
				ordinaryStart := anchor
				if event.HasReplacement || event.HasReplacementUTF16 {
					ordinaryStart = compositionReplacementStart(base, event)
				}
				ordinaryBase := base
				value := []rune(doc.Editor.String())
				if marked.Start >= 0 && marked.Start <= marked.End && marked.End <= len(value) {
					ordinaryBase = string(value[:marked.Start]) + string(value[marked.End:])
				}
				state.provisional = provisionalComposition{doc: doc, base: base, ordinaryBase: ordinaryBase, anchor: anchor, ordinaryStart: ordinaryStart}
			}
			return app.TextStateOf(doc.Editor), true
		}
	}
	if state.provisional.doc == doc {
		state.provisional = provisionalComposition{}
	}
	return app.TextStateOf(doc.Editor), true
}

func (state *model) compositionFrame(doc *document.Document) (string, int, string, int) {
	if state.provisional.doc == doc {
		frame := state.provisional
		return frame.base, frame.anchor, frame.ordinaryBase, frame.ordinaryStart
	}
	if doc == nil || doc.Editor == nil {
		return "", 0, "", 0
	}
	value, start := doc.Editor.String(), doc.Editor.Selection().Start
	return value, start, value, start
}

func compositionReplacementStart(base string, event app.TextEvent) int {
	source := text.NewEditor(base)
	if event.HasReplacementUTF16 {
		units := event.ReplacementUTF16.Start
		index := source.UTF16ToRuneIndex(units)
		if units < 0 || source.RuneIndexToUTF16(index) != units {
			return source.RuneCount()
		}
		return index
	}
	return min(max(min(event.Replacement.Start, event.Replacement.End), 0), source.RuneCount())
}

// compositionTerminalRejected reports whether a terminal tagged event must be
// refused before the rectangle path rolls back its live tentative for the
// per-row edit. A rejected terminal mutates nothing in the editor, so cancelling
// first would destroy a live preedit for an event that the editor itself never
// applied. The text and raw UTF-16 checks mirror the editor's public rejection
// contract; the negative rune range is malformed producer input that the native
// bridge never emits and that the rectangle path cannot resolve, so it is
// refused instead of being clamped by the editor's normalizing result path.
func compositionTerminalRejected(event app.TextEvent) bool {
	switch event.CompositionPhase {
	case text.CompositionCommit, text.CompositionCancel, text.CompositionEmptyCommit:
		return false
	case text.CompositionResult:
	default:
		return true
	}
	if !utf8.ValidString(event.Text) {
		return true
	}
	if event.HasReplacementUTF16 {
		return event.ReplacementUTF16.Start < 0 || event.ReplacementUTF16.Length < 0
	}
	if event.HasReplacement {
		return event.Replacement.Start < 0 || event.Replacement.End < 0
	}
	return false
}

// applyMacroComposition owns the normal document composition path used by the
// macro editor and by multi-selection fallback. Provisional updates leave
// bookmarks, select anchors, completion and macro recording alone; one terminal
// event applies their net effect exactly once.
func (state *model) applyMacroComposition(doc *document.Document, event app.TextEvent) (app.TextState, bool) {
	if doc == nil || doc.Editor == nil {
		return app.TextState{}, false
	}
	if state.isReadOnly(doc) {
		state.cancelDocumentComposition(doc)
		return app.TextStateOf(doc.Editor), false
	}
	base, anchor, ordinaryBase, ordinaryStart := state.compositionFrame(doc)
	before := app.TextStateOf(doc.Editor)
	selectedDirect := state.provisional.doc != doc && before.Selection.Start != before.Selection.End
	composition, wasComposing := doc.Editor.Composition()
	committed := ""
	switch event.CompositionPhase {
	case text.CompositionResult:
		committed = event.Text
	case text.CompositionCommit:
		runes := []rune(before.Value)
		if wasComposing && composition.Start >= 0 && composition.End <= len(runes) && composition.Start <= composition.End {
			committed = string(runes[composition.Start:composition.End])
		}
	}
	next, handled := state.applyDocumentCompositionEvent(doc, event)
	if !handled {
		return before, false
	}
	if event.CompositionPhase == text.CompositionUpdate && doc.Editor.CompositionActive() {
		return next, true
	}
	after := doc.Editor.String()
	if after != base || ordinaryBase != base {
		state.bookmarkLines(doc)
		if event.CompositionPhase == text.CompositionResult && (event.HasReplacement || event.HasReplacementUTF16) {
			if ordinaryBase != base {
				state.relocateSelectAnchors(doc, base, ordinaryBase, ordinaryStart)
				state.relocateSelectAnchors(doc, ordinaryBase, after, compositionReplacementStart(ordinaryBase, event))
			} else {
				state.relocateSelectAnchors(doc, base, after, compositionReplacementStart(base, event))
			}
		} else {
			state.relocateSelectAnchors(doc, base, after, anchor)
		}
		if state.wordCompletion != nil && state.wordCompletion.doc == doc {
			state.updateWordCompletionAfterInput(doc)
		}
	}
	explicitChange := event.HasReplacement && event.Replacement.Start != event.Replacement.End ||
		event.HasReplacementUTF16 && event.ReplacementUTF16.Length != 0
	if state.macroRecording && (ordinaryBase != base ||
		selectedDirect && (after != base || event.CompositionPhase == text.CompositionResult && event.Text != "") ||
		(event.HasReplacement || event.HasReplacementUTF16) && (after != base || event.Text != "" || explicitChange) ||
		(event.CompositionPhase == text.CompositionUpdate || event.CompositionPhase == text.CompositionCancel ||
			event.CompositionPhase == text.CompositionEmptyCommit) && after != base) {
		state.cancelMacroUnsupported("composition replacement")
	}
	if event.CompositionPhase == text.CompositionUpdate || event.CompositionPhase == text.CompositionCancel {
		return next, true
	}
	if state.macroRecording && committed != "" {
		state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: app.TextInsert, Text: committed})
	}
	return next, true
}
