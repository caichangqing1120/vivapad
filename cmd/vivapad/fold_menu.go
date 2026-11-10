package main

import (
	"strconv"

	"github.com/vivaui/viva/ui"
)

func (state *model) paintFoldLevelMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(220), size.Width-8)
	x := min(parentRight, size.Width-width-4)
	rowHeight := float32(25)
	height := 8*rowHeight + 8
	y := max(menuHeight, min(menuHeight+4+15*20, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index := range 8 {
		label := "Fold Level " + strconv.Itoa(index+1)
		if state.foldMenuUnfold {
			label = "Unfold Level " + strconv.Itoa(index+1)
		}
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*rowHeight, Width: width - 8, Height: rowHeight}
		id := foldLevelScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, label, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, label, ui.HitOptions{Role: ui.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		command := "Fold Level " + strconv.Itoa(selected+1)
		if state.foldMenuUnfold {
			command = "Unfold Level " + strconv.Itoa(selected+1)
		}
		state.command(command)
	}
}
