package main

import "github.com/vivaui/viva/gfx"

var changeHistoryItems = [...]struct{ label, command string }{
	{"Go to Next Change", "Next Change"},
	{"Go to Previous Change", "Previous Change"},
	{"Clear Change History", "Clear Change History"},
}

func (state *model) paintChangeHistoryMenu(canvas *gfx.Canvas, size gfx.Size, parentRight, parentY float32) {
	width := min(float32(280), size.Width-parentRight-4)
	x := parentRight
	if width < 175 {
		width = min(280, size.Width-8)
		x = max(2, parentRight-width)
	}
	height := float32(len(changeHistoryItems))*27 + 8
	y := min(max(menuHeight, parentY), max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(changeHistoryScope.At(len(changeHistoryItems)), panel, "Change History", gfx.HitOptions{Role: gfx.RoleMenu})
	unavailable := ""
	if _, err := state.space.Current().ChangeHistory(); err != nil {
		unavailable = err.Error()
	}
	selected := -1
	for index, item := range changeHistoryItems {
		rect := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := changeHistoryScope.At(index)
		enabled := unavailable == "" || item.command == "Clear Change History"
		if (canvas.Hovered() == id || canvas.FocusVisible() == id) && enabled {
			canvas.Fill(rect, palette.hover)
		}
		color := palette.text
		if !enabled {
			color = palette.muted
		}
		label := canvas.FitLine(item.label, 11, gfx.WeightRegular, rect.Width-18)
		smallLabel(canvas, label, gfx.Rect{X: rect.X + 9, Y: rect.Y, Width: rect.Width - 18, Height: rect.Height}, color, 11)
		canvas.HitWithOptions(id, rect, item.label, gfx.HitOptions{Role: gfx.RoleMenuItem, Disabled: !enabled})
		if enabled && canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if unavailable != "" {
		state.notice = unavailable
	}
	if selected >= 0 {
		state.command(changeHistoryItems[selected].command)
	}
}
