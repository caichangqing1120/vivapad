package main

import "github.com/vivaui/viva/gfx"

func (state *model) paintViewMenu(canvas *gfx.Canvas, size gfx.Size, origin float32) {
	commands := menuCommands[3]
	width := min(float32(260), size.Width-8)
	x := min(origin, size.Width-width-2)
	available := size.Height - menuHeight - statusHeight - 8
	rowHeight := float32(27)
	visible := len(commands)
	controls := float32(0)
	if float32(visible)*rowHeight+8 > available {
		rowHeight = 24
		if float32(visible)*rowHeight+8 > available {
			rowHeight, controls = 20, 44
			visible = min(len(commands), max(1, int((available-controls-8)/rowHeight)))
		}
	}
	state.viewMenuVisible = visible
	state.viewMenuOffset = min(max(0, state.viewMenuOffset), len(commands)-visible)
	height := float32(visible)*rowHeight + controls + 8
	y := menuHeight
	canvas.PushLayer(2)
	canvas.PushOverlayClip(gfx.Rect{Width: size.Width, Height: size.Height})
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(viewMenuPanelID, panel, "View menu", gfx.HitOptions{Role: gfx.RoleMenu})
	if controls != 0 {
		for _, control := range []struct {
			id    gfx.ID
			icon  gfx.Icon
			label string
			y     float32
			step  int
		}{
			{viewMenuPreviousID, upIcon, "Previous View commands", y + 4, -1},
			{viewMenuNextID, downIcon, "Next View commands", y + height - 26, 1},
		} {
			bounds := gfx.Rect{X: x + 4, Y: control.y, Width: width - 8, Height: 22}
			canvas.DrawIcon(control.icon, gfx.Rect{X: bounds.X + bounds.Width/2 - 8, Y: bounds.Y + 3, Width: 16, Height: 16}, palette.muted)
			if hitButton(canvas, control.id, bounds, control.label) {
				state.viewMenuOffset = min(max(0, state.viewMenuOffset+control.step), len(commands)-visible)
			}
		}
	}
	rows := gfx.Rect{X: x + 4, Y: y + 4 + controls/2, Width: width - 8, Height: float32(visible) * rowHeight}
	canvas.Clip(rows)
	chosen := ""
	for row := 0; row < visible; row++ {
		index := state.viewMenuOffset + row
		command := commands[index]
		item := gfx.Rect{X: rows.X, Y: rows.Y + float32(row)*rowHeight, Width: rows.Width, Height: rowHeight}
		id := menuItemScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		checked := state.menuSelected(command)
		if checked {
			canvas.DrawIcon(checkIcon, gfx.Rect{X: item.X + 6, Y: item.Y + (rowHeight-14)/2, Width: 14, Height: 14}, palette.blue)
		}
		switch command {
		case "Show Symbol", "Tab", "Fold Level", "Unfold Level":
			canvas.DrawIcon(rightIcon, gfx.Rect{X: item.X + item.Width - 20, Y: item.Y + (rowHeight-14)/2, Width: 14, Height: 14}, palette.muted)
		}
		label := canvas.FitLine(command, 11, gfx.WeightRegular, item.Width-35)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 27, Y: item.Y, Width: item.Width - 34, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem, Checked: checked})
		if canvas.Clicked(id) {
			chosen = command
		}
	}
	canvas.Unclip()
	canvas.PopOverlayClip()
	canvas.PopLayer()
	if chosen != "" {
		state.command(chosen)
	}
	if state.menuOpen == 3 {
		if state.symbolMenuOpen {
			state.paintSymbolMenu(canvas, size, x+width)
		}
		if state.viewTabMenuOpen {
			state.paintViewTabMenu(canvas, size, x+width)
		}
		if state.foldMenuOpen {
			state.paintFoldLevelMenu(canvas, size, x+width)
		}
	}
}
