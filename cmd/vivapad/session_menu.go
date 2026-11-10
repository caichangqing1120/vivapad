package main

import "github.com/vivaui/viva/ui"

var sessionCommands = [...]string{"Load Session...", "Save Session..."}

func (state *model) paintSessionMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(220), size.Width-parentRight-4)
	height := float32(len(sessionCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+13*fileMenuRowHeight, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range sessionCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := sessionScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, command, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, ui.HitOptions{Role: ui.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(sessionCommands[selected])
	}
}
