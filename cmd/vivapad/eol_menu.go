package main

import "github.com/vivaui/viva/ui"

var eolCommands = [...]string{"Windows (CR LF)", "Unix (LF)", "Macintosh (CR)"}

func (state *model) paintEOLMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(230), size.Width-parentRight-4)
	height := float32(len(eolCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+20*16, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range eolCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := eolScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		checked := state.menuSelected(command)
		if checked {
			canvas.DrawIcon(checkIcon, ui.Rect{X: item.X + 5, Y: item.Y + 6, Width: 14, Height: 14}, palette.blue)
		}
		label := canvas.FitLine(command, 11, ui.WeightRegular, item.Width-35)
		smallLabel(canvas, label, ui.Rect{X: item.X + 25, Y: item.Y, Width: item.Width - 30, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, ui.HitOptions{Role: ui.RoleMenuItem, Checked: checked})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(eolCommands[selected])
	}
}
