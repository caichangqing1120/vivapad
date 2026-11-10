package main

import "github.com/vivaui/viva/ui"

var selectedSearchCommands = [...]string{
	"Select and Find Next", "Select and Find Previous",
	"Find (Volatile) Next", "Find (Volatile) Previous",
}

func (state *model) paintSelectedSearchMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(254), size.Width-parentRight-4)
	height := float32(len(selectedSearchCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+18*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range selectedSearchCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := selectedSearchScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(command, 11, ui.WeightRegular, item.Width-18)
		smallLabel(canvas, label, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, ui.HitOptions{Role: ui.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(selectedSearchCommands[selected])
	}
}
