package main

import "github.com/vivaui/viva/gfx"

var symbolCommands = [...]string{
	"Show Space and Tab", "Show End of Line", "Show Non-Printing Characters",
	"Show Control Characters & Unicode EOL", "Show All Characters", "Show Indent Guide", "Show Wrap Symbol",
}

func (state *model) paintSymbolMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	palette := state.palette()
	width := min(float32(230), size.Width-parentRight-4)
	height := float32(len(symbolCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+9*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.popup)
	selected := -1
	for index, command := range symbolCommands {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := symbolScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		checked := state.menuSelected(command)
		if checked {
			canvas.DrawIcon(checkIcon, gfx.Rect{X: item.X + 4, Y: item.Y + 6, Width: 14, Height: 14}, palette.accent)
		}
		label := canvas.FitLine(command, 11, gfx.WeightRegular, item.Width-35)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 24, Y: item.Y, Width: item.Width - 30, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem, Checked: checked})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(symbolCommands[selected])
	}
}
