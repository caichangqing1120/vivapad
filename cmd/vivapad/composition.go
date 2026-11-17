package main

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/macro"
)

type provisionalComposition struct {
	doc           *document.Document
	base          string
	ordinaryBase  string
	anchor        int
	ordinaryStart int
}

type compositionOwner struct {
	doc    *document.Document
	editor *text.Editor
	target gfx.ID
}

func (state *model) reconcileCompositionOwner(target gfx.ID, input *app.TextInput) {
	var next compositionOwner
	next.target = target
	if input != nil {
		next.editor = input.Editor
		if target == editorID || target == rightEditorID {
			pane := 0
			if target == rightEditorID {
				pane = 1
			}
			next.doc = state.paneDocument(pane)
			if next.doc != nil {
				next.editor = next.doc.Editor
			}
		}
	}
	if state.compositionOwner == next {
		return
	}
	state.cancelCompositionOwner()
	state.compositionOwner = next
}

func (state *model) cancelCompositionOwner() {
	owner := state.compositionOwner
	if owner.doc != nil && owner.doc.Editor == owner.editor && owner.editor.TentativeCompositionEnabled() {
		state.applyPublicComposition(owner.doc, app.TextEvent{CompositionPhase: text.CompositionCancel})
	} else if owner.editor != nil && owner.editor.TentativeCompositionEnabled() {
		owner.editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel})
	}
	state.provisional = provisionalComposition{}
	state.compositionOwner = compositionOwner{}
	if owner.target != (gfx.ID{}) || owner.editor != nil {
		state.compositionGeneration++
	}
}

func (state *model) pointerCompositionAway(target gfx.ID) {
	if state.compositionOwner.editor == nil || state.compositionOwner.target == target {
		return
	}
	state.cancelCompositionOwner()
	state.compositionOwner.target = target
}

func (state *model) cancelDocumentComposition(doc *document.Document) {
	if doc == nil || doc.Editor == nil {
		return
	}
	if state.compositionOwner.doc == doc {
		state.cancelCompositionOwner()
		return
	}
	if doc.Editor.TentativeCompositionEnabled() {
		state.applyPublicComposition(doc, app.TextEvent{CompositionPhase: text.CompositionCancel})
	}
}

func (state *model) applyPublicComposition(doc *document.Document, event app.TextEvent) app.TextState {
	before := app.TextStateOf(doc.Editor)
	base, anchor := before.Value, doc.Editor.Selection().Start
	selectedDirect := state.provisional.doc != doc && before.Selection.Start != before.Selection.End
	ordinaryBase, ordinaryStart := base, before.Selection.Start
	if state.provisional.doc == doc {
		base, anchor = state.provisional.base, state.provisional.anchor
		ordinaryBase, ordinaryStart = state.provisional.ordinaryBase, state.provisional.ordinaryStart
	}
	marked, composing := doc.Editor.Composition()
	committed := ""
	switch event.CompositionPhase {
	case text.CompositionResult:
		committed = event.Text
	case text.CompositionCommit:
		if composing {
			runes := []rune(before.Value)
			if marked.Start >= 0 && marked.Start <= marked.End && marked.End <= len(runes) {
				committed = string(runes[marked.Start:marked.End])
			}
		}
	}
	if !doc.Editor.ApplyComposition(text.CompositionEvent{
		Phase:               event.CompositionPhase,
		Text:                event.Text,
		Selected:            text.Range{Start: event.MarkedSelection.Start, End: event.MarkedSelection.End},
		Replacement:         text.Range{Start: event.Replacement.Start, End: event.Replacement.End},
		HasReplacement:      event.HasReplacement,
		ReplacementUTF16:    event.ReplacementUTF16,
		HasReplacementUTF16: event.HasReplacementUTF16,
	}) {
		return before
	}
	if event.CompositionPhase == text.CompositionUpdate {
		if marked, active := doc.Editor.Composition(); active {
			if state.provisional.doc != doc {
				if event.HasReplacement || event.HasReplacementUTF16 {
					ordinaryStart = compositionReplacementStart(base, event)
				}
				value := []rune(doc.Editor.String())
				if marked.Start >= 0 && marked.Start <= marked.End && marked.End <= len(value) {
					ordinaryBase = string(value[:marked.Start]) + string(value[marked.End:])
				}
				state.provisional = provisionalComposition{
					doc: doc, base: base, ordinaryBase: ordinaryBase, anchor: anchor,
					ordinaryStart: ordinaryStart,
				}
			}
			return app.TextStateOf(doc.Editor)
		}
	}
	if state.provisional.doc == doc {
		state.provisional = provisionalComposition{}
	}
	after := doc.Editor.String()
	if after != base || ordinaryBase != base {
		state.bookmarkLines(doc)
		editStart := anchor
		if event.CompositionPhase == text.CompositionResult && (event.HasReplacement || event.HasReplacementUTF16) {
			if ordinaryBase != base {
				state.relocateSelectAnchors(doc, base, ordinaryBase, ordinaryStart)
				editStart = compositionReplacementStart(ordinaryBase, event)
				state.relocateSelectAnchors(doc, ordinaryBase, after, editStart)
			} else {
				editStart = compositionReplacementStart(base, event)
				state.relocateSelectAnchors(doc, base, after, editStart)
			}
		} else {
			state.relocateSelectAnchors(doc, base, after, editStart)
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
		return app.TextStateOf(doc.Editor)
	}
	if state.macroRecording && committed != "" {
		state.recordMacro(doc, macro.Step{Kind: macro.TextStep, TextKind: app.TextInsert, Text: committed})
	}
	return app.TextStateOf(doc.Editor)
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
