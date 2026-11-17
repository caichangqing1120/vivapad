package main

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"slices"
)

func (state *model) redactSelection(bullet bool) {
	doc := state.paneDocument(state.activePane)
	if state.isReadOnly(doc) {
		state.notice = "Read-only document"
		return
	}
	selection := state.rectangles[state.activePane]
	activeRectangle := selection.active && selection.doc == doc && selection.value == doc.Editor.String()
	multi := state.multiSelections[state.activePane]
	activeMulti := !activeRectangle && multi.active && multi.doc == doc && multi.value == doc.Editor.String()
	var ranges []text.Range
	if activeRectangle {
		for _, row := range doc.RectangleRows(state.rectangleForPane(state.activePane)) {
			if row.Range.Start < row.Range.End {
				ranges = append(ranges, row.Range)
			}
		}
	} else if activeMulti {
		ranges = append(ranges, multi.ranges...)
		slices.SortFunc(ranges, func(left, right text.Range) int { return left.Start - right.Start })
	} else if selected := doc.Editor.Selection(); selected.Start < selected.End {
		ranges = append(ranges, selected)
	}
	if len(ranges) == 0 {
		return
	}
	before := doc.Editor.String()
	var changed bool
	var err error
	state.editWithBookmarkAnchor(doc, func() { changed, err = doc.RedactRanges(ranges, bullet) })
	if err != nil {
		state.notice = "Redact failed: " + err.Error()
		return
	}
	if !changed {
		return
	}
	if activeRectangle {
		state.rectangles[state.activePane] = rectangleSelection{}
		state.recordRectangleTransition(state.activePane, doc, before, doc.Editor.String(), selection)
	}
	if activeMulti {
		state.multiSelections[state.activePane] = multiSelectionState{}
		state.recordMultiSelectionTransition(state.activePane, doc, before, doc.Editor.String(), multi)
	}
	if state.macroRecording {
		state.cancelMacroUnsupported("redaction")
	}
}

func (state *model) paintOnSelectionMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	width := min(float32(280), size.Width-parentRight-4)
	height := float32(3*27 + 8)
	x := parentRight
	y := min(menuHeight+4+23*24, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(onSelectionScope.At(3), panel, "On Selection", gfx.HitOptions{Role: gfx.RoleMenu})
	for index, item := range []struct {
		label, command string
	}{
		{"Open File", "Open File from Selection"},
		{"Open Containing Folder", "Open Containing Folder from Selection"},
		{"Redact Selection (Shift: ●)", "Redact Selection"},
	} {
		bounds := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := onSelectionScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(bounds, palette.hover)
		}
		smallLabel(canvas, item.label, bounds.Inset(gfx.All(8)), palette.text, 11)
		canvas.HitWithOptions(id, bounds, item.label, gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(id) {
			command := item.command
			if index == 2 && state.menuPointerShift {
				command = "Redact Selection with Bullets"
			}
			state.command(command)
		}
	}
	canvas.PopLayer()
}
