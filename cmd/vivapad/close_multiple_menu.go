package main

import "github.com/vivaui/viva/ui"

var closeMultipleCommands = [...]string{
	"Close All but Active Document", "Close All but Pinned Documents", "Close All to the Left", "Close All to the Right", "Close All Unchanged",
}

func (state *model) paintCloseMultipleMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(290), size.Width-parentRight-4)
	height := float32(len(closeMultipleCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+15*fileMenuRowHeight, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range closeMultipleCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := closeMultipleScope.At(index)
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
		state.command(closeMultipleCommands[selected])
	}
}
