package main

import (
	"runtime"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

const (
	textDirectionRTL = "Text Direction RTL"
	textDirectionLTR = "Text Direction LTR"
)

func editorDirection(doc *document.Document) gfx.TextDirection {
	if doc.RTL {
		return gfx.DirectionRTL
	}
	return gfx.DirectionLTR
}

func (state *model) restoreSessionDirection(rtl map[string]bool) {
	for _, doc := range state.space.Documents {
		doc.RTL = rtl[doc.Path]
	}
}

func (state *model) setTextDirection(rtl bool) {
	doc := state.paneDocument(state.activePane)
	doc.RTL = rtl
	state.menuOpen, state.menuOverflowOpen = -1, false
	state.requestEditorCommandFocus()
	state.invalidator.Invalidate()
}

func (state *model) moveShapedHorizontal(pane int, doc *document.Document, event gfx.KeyEvent) bool {
	if (event.Key != gfx.KeyLeft && event.Key != gfx.KeyRight) || doc.Editor.CompositionActive() {
		return false
	}
	right := event.Key == gfx.KeyRight
	extend := event.Modifiers&gfx.ModifierShift != 0
	modifiers := event.Modifiers &^ gfx.ModifierShift
	word := modifiers == gfx.ModifierControl || runtime.GOOS == "darwin" && modifiers == gfx.ModifierAlt
	if modifiers != 0 && !word {
		return false
	}
	return state.moveTextDirection(pane, doc, right, extend, word)
}

func (state *model) moveShapedTextCommand(pane int, doc *document.Document, event app.TextEvent) bool {
	if event.CompositionPhase != text.CompositionNone || event.Kind != app.TextCommandEvent || doc.Editor.CompositionActive() {
		return false
	}
	// Existing LTR macros record native text commands and replay them without a
	// viewport. Use that same path when recording, including on mixed rows.
	if state.macroRecording && !doc.RTL {
		return false
	}
	switch event.Command {
	case app.TextMoveLeft, app.TextMoveRight, app.TextMoveLeftAndModifySelection, app.TextMoveRightAndModifySelection:
		right := event.Command == app.TextMoveRight || event.Command == app.TextMoveRightAndModifySelection
		extend := event.Command == app.TextMoveLeftAndModifySelection || event.Command == app.TextMoveRightAndModifySelection
		return state.moveTextDirection(pane, doc, right, extend, false)
	case app.TextMoveWordLeft, app.TextMoveWordRight, app.TextMoveWordLeftAndModifySelection, app.TextMoveWordRightAndModifySelection:
		right := event.Command == app.TextMoveWordRight || event.Command == app.TextMoveWordRightAndModifySelection
		extend := event.Command == app.TextMoveWordLeftAndModifySelection || event.Command == app.TextMoveWordRightAndModifySelection
		return state.moveTextDirection(pane, doc, right, extend, true)
	}
	return false
}

func (state *model) moveTextDirection(pane int, doc *document.Document, right, extend, word bool) bool {
	if word {
		if !doc.RTL {
			return false
		}
		doc.Editor.MoveWord(!right, extend)
	} else {
		selection := doc.Editor.Selection()
		editor, value := state.codeEditorForPane(pane), doc.Editor.String()
		if !extend && selection.Start != selection.End {
			if at, ok := editor.CollapseHorizontal(value, doc.Editor.ByteOffset(selection.Start), doc.Editor.ByteOffset(selection.End), right); ok {
				doc.Editor.MoveTo(doc.Editor.RuneIndex(at), false)
			} else {
				if !doc.RTL {
					return false
				}
				next := selection.End
				if right {
					next = selection.Start
				}
				doc.Editor.MoveTo(next, false)
			}
		} else {
			at := doc.Editor.ByteOffset(doc.Editor.Caret())
			next, ok := editor.MoveHorizontal(value, at, right)
			if !ok {
				return false
			}
			doc.Editor.MoveTo(doc.Editor.RuneIndex(next), extend)
		}
	}
	if state.macroRecording {
		state.cancelMacroUnsupported("movement")
	}
	return true
}
