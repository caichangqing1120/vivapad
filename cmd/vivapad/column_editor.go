package main

import (
	"strconv"
	"strings"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func (state *model) openColumnDialog() {
	state.columnDialogOpen, state.columnNumber, state.columnFocus = true, false, true
	state.columnText = text.NewEditor("")
	state.columnInitial, state.columnIncrement, state.columnRepeat = text.NewEditor("0"), text.NewEditor("1"), text.NewEditor("1")
	state.columnBase, state.columnUppercase, state.columnLeading, state.columnError = 10, false, document.ColumnLeadingNone, ""
}

func (state *model) columnField(id ui.ID) bool {
	return id == columnTextID || id == columnInitialID || id == columnIncrementID || id == columnRepeatID
}

func (state *model) changeColumnBase(base int) {
	if state.columnBase == base {
		return
	}
	fields := []*text.Editor{state.columnInitial, state.columnIncrement, state.columnRepeat}
	values := [3]int64{}
	for index, field := range fields {
		value, err := strconv.ParseInt(field.String(), state.columnBase, 32)
		if err != nil || value < 0 {
			state.columnError = "Correct the number fields before changing format"
			return
		}
		values[index] = value
	}
	state.columnBase, state.columnError = base, ""
	for index, value := range values {
		representation := strconv.FormatInt(value, base)
		if state.columnUppercase && base == 16 {
			representation = strings.ToUpper(representation)
		}
		fields[index] = text.NewEditor(representation)
	}
	state.columnInitial, state.columnIncrement, state.columnRepeat = fields[0], fields[1], fields[2]
}

func (state *model) applyColumnDialog() {
	if !state.columnDialogOpen {
		return
	}
	doc := state.paneDocument(state.activePane)
	if state.isReadOnly(doc) {
		state.columnError = "Read-only document"
		return
	}
	before := doc.Editor.String()
	options := document.ColumnEdit{
		Text: state.columnText.String(), Number: state.columnNumber,
		Initial: state.columnInitial.String(), Increment: state.columnIncrement.String(), Repeat: state.columnRepeat.String(),
		Base: state.columnBase, Uppercase: state.columnUppercase, Leading: state.columnLeading,
	}
	var changed bool
	var editError error
	rect := state.rectangles[state.activePane]
	state.editWithBookmarkAnchor(doc, func() {
		if rect.active && rect.doc == doc && rect.value == before {
			changed, editError = doc.EditRectangle(state.rectangleForPane(state.activePane), options)
		} else {
			changed, editError = doc.EditColumn(options, state.tabWidth)
		}
	})
	if editError != nil {
		state.columnError = editError.Error()
		return
	}
	if changed {
		state.relocateSelectAnchors(doc, before, doc.Editor.String(), -1)
		state.notice = "Column inserted"
	}
	state.rectangles[state.activePane] = rectangleSelection{}
	if changed && rect.active && rect.doc == doc && rect.value == before {
		state.recordRectangleTransition(state.activePane, doc, before, doc.Editor.String(), rect)
	}
	state.columnDialogOpen = false
}
