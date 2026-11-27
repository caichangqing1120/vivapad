package main

import "github.com/vivaui/viva/gfx"

var eolCommands = [...]string{"Windows (CR LF)", "Unix (LF)", "Macintosh (CR)"}

func (state *model) paintEOLMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	palette := state.palette()
	width := min(float32(230), size.Width-parentRight-4)
	height := float32(len(eolCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+20*16, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.popup)
	selected := -1
	for index, command := range eolCommands {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := eolScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		checked := state.menuSelected(command)
		if checked {
			canvas.DrawIcon(checkIcon, gfx.Rect{X: item.X + 5, Y: item.Y + 6, Width: 14, Height: 14}, palette.accent)
		}
		label := canvas.FitLine(command, 11, gfx.WeightRegular, item.Width-35)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 25, Y: item.Y, Width: item.Width - 30, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem, Checked: checked})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(eolCommands[selected])
	}
}
