package main

import (
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

var caseCommands = [...]string{
	"UPPERCASE", "lowercase", "Proper Case", "Proper Case (blend)",
	"Sentence case", "Sentence case (blend)", "Invert Case", "Random Case",
}

func (state *model) convertSelectedCase(doc *document.Document, mode document.CaseMode, notice string) {
	selection := doc.Editor.Selection()
	if selection.Start == selection.End {
		state.notice = "Select text to convert"
	} else if doc.ConvertCase(mode) {
		state.notice = notice
	} else {
		state.notice = "Selection unchanged"
	}
}

func (state *model) paintCaseMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	palette := state.palette()
	width := min(float32(220), size.Width-parentRight-4)
	height := float32(len(caseCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+11*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.popup)
	selected := -1
	for index, command := range caseCommands {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := caseScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, command, gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(caseCommands[selected])
	}
}
