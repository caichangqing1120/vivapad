package main

import (
	"runtime"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

type rectangleSelection struct {
	doc                         *document.Document
	anchor, caret               int
	anchorVirtual, caretVirtual int
	value                       string
	copyText                    string
	pending, active, composing  bool
	copyValid                   bool
}

func (state *model) rectangleForPane(pane int) document.Rectangle {
	selection := state.rectangles[pane]
	return document.Rectangle{
		Anchor: selection.anchor, Caret: selection.caret,
		AnchorVirtual: selection.anchorVirtual, CaretVirtual: selection.caretVirtual, TabWidth: state.tabWidth,
	}
}

func (state *model) beginEndColumnSelect() {
	pane := state.activePane
	state.multiSelections[pane] = multiSelectionState{}
	doc := state.paneDocument(pane)
	selection := &state.rectangles[pane]
	state.selectAnchors[pane] = selectAnchor{}
	if selection.doc != doc || !selection.pending || selection.value != doc.Editor.String() {
		*selection = rectangleSelection{doc: doc, anchor: doc.Editor.Caret(), caret: doc.Editor.Caret(), value: doc.Editor.String(), pending: true}
		state.notice = "Column selection start set"
		return
	}
	selection.caret, selection.caretVirtual = doc.Editor.Caret(), 0
	selection.pending, selection.active = false, true
	selection.copyValid = false
	state.notice = "Column selection completed"
}

func (state *model) extendRectangleKey(pane int, event ui.KeyEvent) bool {
	if event.Modifiers != ui.ModifierAlt|ui.ModifierShift {
		return false
	}
	switch event.Key {
	case ui.KeyLeft, ui.KeyRight, ui.KeyUp, ui.KeyDown:
	default:
		return false
	}
	doc := state.paneDocument(pane)
	selection := &state.rectangles[pane]
	if runtime.GOOS == "darwin" && !selection.active && !selection.pending {
		return false
	}
	if selection.doc != doc || !selection.active || selection.value != doc.Editor.String() {
		*selection = rectangleSelection{doc: doc, anchor: doc.Editor.Caret(), caret: doc.Editor.Caret(), value: doc.Editor.String(), active: true}
		state.selectAnchors[pane] = selectAnchor{}
	}
	switch event.Key {
	case ui.KeyLeft:
		if selection.caretVirtual > 0 {
			selection.caretVirtual--
		} else {
			doc.Editor.MoveLeft(false)
		}
	case ui.KeyRight:
		value := doc.Editor.String()
		at := doc.Editor.ByteOffset(doc.Editor.Caret())
		if selection.caretVirtual > 0 || at == len(value) || value[at] == '\n' {
			selection.caretVirtual++
		} else {
			doc.Editor.MoveRight(false)
		}
	case ui.KeyUp, ui.KeyDown:
		movement := components.TextareaMoveUp
		if event.Key == ui.KeyDown {
			movement = components.TextareaMoveDown
		}
		value := doc.Editor.String()
		at := doc.Editor.ByteOffset(doc.Editor.Caret())
		currentStart := strings.LastIndexByte(value[:at], '\n') + 1
		goal := rectangleVisualColumn(value[currentStart:at], state.tabWidth) + selection.caretVirtual
		next := state.codeEditorForPane(pane).Move(value, at, movement)
		doc.Editor.MoveTo(doc.Editor.RuneIndex(next), false)
		selection.caretVirtual = 0
		start := strings.LastIndexByte(value[:next], '\n') + 1
		if next == len(value) || value[next] == '\n' {
			selection.caretVirtual = max(0, goal-rectangleVisualColumn(value[start:next], state.tabWidth))
		}
	}
	selection.caret = doc.Editor.Caret()
	selection.copyValid = false
	return true
}

func rectangleVisualColumn(value string, tabWidth int) int {
	column := 0
	for _, character := range value {
		if character == '\t' {
			column += tabWidth - column%tabWidth
		} else {
			column += text.CellWidth(character)
		}
	}
	return column
}

func (state *model) paintRectangleSelection(snapshot *components.CodeEditorSnapshot, doc *document.Document, pane int) {
	selection := &state.rectangles[pane]
	if selection.doc != nil && selection.doc != doc {
		*selection = rectangleSelection{}
		return
	}
	if !selection.active {
		return
	}
	if selection.composing {
		return
	}
	if selection.value != doc.Editor.String() {
		*selection = rectangleSelection{}
		return
	}
	snapshot.Caret = doc.Editor.ByteOffset(selection.caret)
	snapshot.SelectionStart, snapshot.SelectionEnd = snapshot.Caret, snapshot.Caret
	rows := doc.RectangleRows(state.rectangleForPane(pane))
	for _, row := range rows {
		if row.Range.Start != row.Range.End {
			snapshot.SelectionRanges = append(snapshot.SelectionRanges, text.Span{Start: doc.Editor.ByteOffset(row.Range.Start), End: doc.Editor.ByteOffset(row.Range.End)})
		} else if row.Line != strings.Count(doc.Editor.String()[:doc.Editor.ByteOffset(selection.caret)], "\n") || row.VirtualCells > 0 {
			snapshot.ExtraCarets = append(snapshot.ExtraCarets, components.CodeEditorCaret{Offset: doc.Editor.ByteOffset(row.Range.Start), VirtualCells: row.VirtualCells})
		}
	}
	if len(snapshot.SelectionRanges) > 0 {
		snapshot.ShowCaret = false
	}
}

func (state *model) rectangleTextInput(pane int) *app.TextInput {
	doc := state.paneDocument(pane)
	selection := &state.rectangles[pane]
	if selection.doc != doc || !selection.composing && selection.value != doc.Editor.String() {
		*selection = rectangleSelection{}
		return state.macroTextInput(doc)
	}
	copyText := ""
	if !selection.composing {
		if !selection.copyValid {
			var err error
			selection.copyText, err = doc.RectangleText(state.rectangleForPane(pane))
			selection.copyValid = true
			if err != nil {
				state.notice = err.Error()
			}
		}
		copyText = selection.copyText
	}
	return &app.TextInput{State: app.TextStateOf(doc.Editor), CopyOverride: true, CopyText: copyText, CopyRectangular: true, OnEvent: func(event app.TextEvent) (app.TextState, error) {
		selection := &state.rectangles[pane]
		if state.isReadOnly(doc) {
			state.notice = "Read-only document"
			return app.TextStateOf(doc.Editor), nil
		}
		if event.Kind == app.TextSetMarked && selection.active && selection.doc == doc && (selection.composing || selection.value == doc.Editor.String()) {
			state.editWithBookmarkAnchor(doc, func() { event.Apply(doc.Editor) })
			selection.composing = true
			return app.TextStateOf(doc.Editor), nil
		}
		if selection.composing {
			if event.Kind != app.TextUnmark && event.Kind != app.TextInsert && event.Kind != app.TextPaste {
				*selection = rectangleSelection{}
				return state.macroTextInput(doc).OnEvent(event)
			}
			composition, ok := doc.Editor.Composition()
			if !ok {
				*selection = rectangleSelection{}
				return app.TextStateOf(doc.Editor), nil
			}
			committed := event.Text
			if event.Kind == app.TextUnmark {
				committed = string([]rune(doc.Editor.String())[composition.Start:composition.End])
			}
			doc.Editor.Unmark()
			undone := false
			state.editWithBookmarkAnchor(doc, func() { undone = doc.Editor.Undo() })
			if !undone || doc.Editor.String() != selection.value {
				*selection = rectangleSelection{}
				state.notice = "Column input could not restore preedit"
				return app.TextStateOf(doc.Editor), nil
			}
			selection.composing = false
			if committed == "" {
				return app.TextStateOf(doc.Editor), nil
			}
			event = app.TextEvent{Kind: app.TextInsert, Text: committed}
		}
		if event.Kind == app.TextSetSelection {
			start, end := min(selection.anchor, selection.caret), max(selection.anchor, selection.caret)
			if event.Selection.Start == event.Selection.End && event.Selection.Start == doc.Editor.Caret() ||
				event.Selection.Start == start && event.Selection.End == end {
				return app.TextStateOf(doc.Editor), nil
			}
		}
		if event.Kind == app.TextPasteNoText {
			return app.TextStateOf(doc.Editor), nil
		}
		if event.Kind == app.TextCommandEvent {
			key := ui.KeyUnknown
			switch event.Command {
			case app.TextMoveWordLeftAndModifySelection:
				key = ui.KeyLeft
			case app.TextMoveWordRightAndModifySelection:
				key = ui.KeyRight
			}
			if key != ui.KeyUnknown && state.extendRectangleKey(pane, ui.KeyEvent{Key: key, Modifiers: ui.ModifierAlt | ui.ModifierShift}) {
				return app.TextStateOf(doc.Editor), nil
			}
		}
		if !selection.active || selection.doc != doc || selection.value != doc.Editor.String() {
			*selection = rectangleSelection{}
			return state.macroTextInput(doc).OnEvent(event)
		}
		cutting := event.Kind == app.TextPaste && event.Text == "" && copyText != ""
		deleting := cutting || event.Kind == app.TextCommandEvent && (event.Command == app.TextDeleteBackward || event.Command == app.TextDeleteForward)
		multiline := event.Kind == app.TextPaste && strings.ContainsAny(event.Text, "\r\n")
		if event.Kind != app.TextInsert && event.Kind != app.TextPaste && !deleting {
			*selection = rectangleSelection{}
			return state.macroTextInput(doc).OnEvent(event)
		}
		if !deleting && event.Text == "" {
			return app.TextStateOf(doc.Editor), nil
		}
		if !deleting && (strings.IndexByte(event.Text, 0) >= 0 || !multiline && strings.ContainsAny(event.Text, "\r\n")) {
			state.notice = "Column input must be a single line"
			return app.TextStateOf(doc.Editor), nil
		}
		rect := state.rectangleForPane(pane)
		rows := doc.RectangleRows(rect)
		if len(rows) == 0 {
			*selection = rectangleSelection{}
			return app.TextStateOf(doc.Editor), nil
		}
		before := doc.Editor.String()
		previous := *selection
		anchorLine := strings.Count(before[:doc.Editor.ByteOffset(selection.anchor)], "\n")
		caretLine := strings.Count(before[:doc.Editor.ByteOffset(selection.caret)], "\n")
		var changed bool
		var editError error
		state.editWithBookmarkAnchor(doc, func() {
			if deleting {
				changed, editError = doc.DeleteRectangle(rect, event.Command == app.TextDeleteBackward || cutting)
			} else if multiline {
				changed, editError = doc.PasteRectangleLines(rect, event.Text)
			} else {
				changed, editError = doc.EditRectangle(rect, document.ColumnEdit{Text: event.Text})
			}
		})
		if editError != nil {
			state.notice = editError.Error()
			return app.TextStateOf(doc.Editor), nil
		}
		if multiline {
			*selection = rectangleSelection{}
			if changed {
				state.recordRectangleTransition(pane, doc, before, doc.Editor.String(), previous)
			}
			state.relocateSelectAnchors(doc, before, doc.Editor.String(), -1)
			if state.macroRecording {
				state.cancelMacroUnsupported("column paste")
			}
			return app.TextStateOf(doc.Editor), nil
		}
		if changed || deleting {
			column := rows[0].LeftColumn
			if deleting {
				if rows[0].LeftColumn == rows[0].RightColumn && event.Command == app.TextDeleteBackward {
					column = max(0, column-1)
				}
			} else {
				column += rectangleVisualColumnFrom(event.Text, column, state.tabWidth)
			}
			state.refreshRectangleAfterEdit(pane, doc, before, anchorLine, caretLine, column)
			if changed {
				state.recordRectangleTransition(pane, doc, before, doc.Editor.String(), previous)
			}
			if state.macroRecording {
				state.cancelMacroUnsupported("column input")
			}
		}
		return app.TextStateOf(doc.Editor), nil
	}}
}

func (state *model) refreshRectangleAfterEdit(pane int, doc *document.Document, before string, anchorLine, caretLine, column int) {
	value := doc.Editor.String()
	anchor, anchorVirtual := rectangleCaretAtColumn(doc, value, anchorLine, column, state.tabWidth)
	caret, caretVirtual := rectangleCaretAtColumn(doc, value, caretLine, column, state.tabWidth)
	doc.Editor.MoveTo(caret, false)
	state.rectangles[pane] = rectangleSelection{doc: doc, anchor: anchor, caret: caret, anchorVirtual: anchorVirtual, caretVirtual: caretVirtual, value: value, active: true}
	state.relocateSelectAnchors(doc, before, value, -1)
}

func (state *model) copyRectangle(doc *document.Document, cut bool) {
	pane := state.activePane
	rect := state.rectangleForPane(pane)
	rows := doc.RectangleRows(rect)
	value, err := doc.RectangleText(rect)
	if err != nil {
		state.notice = err.Error()
		return
	}
	if value == "" {
		return
	}
	if err := app.SetRectangularClipboardText(value); err != nil {
		state.notice = err.Error()
		return
	}
	if !cut {
		return
	}
	before := doc.Editor.String()
	selection := state.rectangles[pane]
	anchorLine := strings.Count(before[:doc.Editor.ByteOffset(selection.anchor)], "\n")
	caretLine := strings.Count(before[:doc.Editor.ByteOffset(selection.caret)], "\n")
	var changed bool
	state.editWithBookmarkAnchor(doc, func() { changed, err = doc.DeleteRectangle(rect, true) })
	if err != nil {
		state.notice = err.Error()
		return
	}
	if changed {
		state.refreshRectangleAfterEdit(pane, doc, before, anchorLine, caretLine, rows[0].LeftColumn)
		state.recordRectangleTransition(pane, doc, before, doc.Editor.String(), selection)
		if state.macroRecording {
			state.cancelMacroUnsupported("column cut")
		}
	}
}

func rectangleVisualColumnFrom(value string, start, tabWidth int) int {
	column := start
	for _, character := range value {
		if character == '\t' {
			column += tabWidth - column%tabWidth
		} else {
			column += text.CellWidth(character)
		}
	}
	return column - start
}

func rectangleCaretAtColumn(doc *document.Document, value string, line, target, tabWidth int) (int, int) {
	start := 0
	for index := 0; index < line; index++ {
		next := strings.IndexByte(value[start:], '\n')
		if next < 0 {
			return doc.Editor.RuneCount(), 0
		}
		start += next + 1
	}
	end := len(value)
	if next := strings.IndexByte(value[start:], '\n'); next >= 0 {
		end = start + next
	}
	column := 0
	for offset, character := range value[start:end] {
		if column >= target && text.CellWidth(character) != 0 {
			return doc.Editor.RuneIndex(start + offset), 0
		}
		if character == '\t' {
			column += tabWidth - column%tabWidth
		} else {
			column += text.CellWidth(character)
		}
	}
	return doc.Editor.RuneIndex(end), max(0, target-column)
}
