package main

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
)

func (state *model) paintWindowMenu(canvas *gfx.Canvas, size gfx.Size, origin float32) {
	tabs := state.activeViewTabs()
	count := min(40, len(tabs))
	width := min(float32(300), size.Width-8)
	x := min(origin, size.Width-width-2)
	available := size.Height - menuHeight - statusHeight - 8
	fixed := float32(2*25 + 20 + 8)
	visible := count
	controls := float32(0)
	if fixed+float32(count)*23 > available {
		controls = 44
		visible = max(1, int((available-fixed-controls)/23))
	}
	state.windowRecentVisible = visible
	state.windowRecentOffset = min(max(0, state.windowRecentOffset), max(0, count-visible))
	height := fixed + controls + float32(visible)*23
	y := menuHeight
	canvas.PushLayer(2)
	canvas.PushOverlayClip(gfx.Rect{Width: size.Width, Height: size.Height})
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selectedCommand, selectedIndex := "", -1
	for index, command := range []string{"Sort By", "Windows..."} {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*25, Width: width - 8, Height: 25}
		id := menuItemScope.At(index)
		if canvas.Hovered() == id || index == 0 && state.windowSortMenuOpen {
			canvas.Fill(item, palette.hover)
		}
		if index == 0 {
			canvas.DrawIcon(rightIcon, gfx.Rect{X: item.X + item.Width - 21, Y: item.Y + 5, Width: 16, Height: 16}, palette.muted)
		}
		smallLabel(canvas, command, gfx.Rect{X: item.X + 26, Y: item.Y, Width: item.Width - 48, Height: item.Height}, palette.text, 12)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(id) {
			selectedCommand = command
		}
	}
	heading := gfx.Rect{X: x + 4, Y: y + 54, Width: width - 8, Height: 20}
	canvas.Fill(gfx.Rect{X: heading.X, Y: heading.Y, Width: heading.Width, Height: 1}, palette.border)
	smallLabel(canvas, "Recent Window", gfx.Rect{X: heading.X + 9, Y: heading.Y + 1, Width: heading.Width - 18, Height: 19}, palette.muted, 10)
	if controls != 0 {
		for _, control := range []struct {
			id    gfx.ID
			icon  gfx.Icon
			label string
			y     float32
			step  int
		}{
			{windowRecentPreviousID, upIcon, "Previous windows", y + 74, -1},
			{windowRecentNextID, downIcon, "Next windows", y + height - 26, 1},
		} {
			bounds := gfx.Rect{X: x + 4, Y: control.y, Width: width - 8, Height: 22}
			canvas.DrawIcon(control.icon, gfx.Rect{X: bounds.X + bounds.Width/2 - 8, Y: bounds.Y + 3, Width: 16, Height: 16}, palette.muted)
			canvas.HitWithOptions(control.id, bounds, control.label, gfx.HitOptions{Role: gfx.RoleMenuItem})
			if canvas.Clicked(control.id) {
				state.windowRecentOffset = min(max(0, state.windowRecentOffset+control.step), count-visible)
			}
		}
	}
	rows := gfx.Rect{X: x + 4, Y: y + 74 + controls/2, Width: width - 8, Height: float32(visible) * 23}
	canvas.HitWithOptions(windowRecentPanelID, rows, "Recent windows", gfx.HitOptions{Role: gfx.RoleMenu})
	current := state.paneDocument(state.activePane)
	for row := range visible {
		index := state.windowRecentOffset + row
		if index >= count {
			break
		}
		doc := tabs[index]
		item := gfx.Rect{X: rows.X, Y: rows.Y + float32(row)*23, Width: rows.Width, Height: 23}
		id := windowRecentScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		checked := doc == current
		if checked {
			canvas.DrawIcon(checkIcon, gfx.Rect{X: item.X + 5, Y: item.Y + 4, Width: 14, Height: 14}, palette.blue)
		}
		label := strconv.Itoa(index+1) + "  " + doc.Name
		if doc.Dirty() {
			label += "*"
		}
		fit := canvas.FitLine(label, 11, gfx.WeightRegular, item.Width-40)
		smallLabel(canvas, fit, gfx.Rect{X: item.X + 26, Y: item.Y, Width: item.Width - 34, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, label, gfx.HitOptions{Role: gfx.RoleMenuItem, Checked: checked, Hint: doc.Path})
		if canvas.Clicked(id) {
			selectedIndex = index
		}
	}
	canvas.PopOverlayClip()
	canvas.PopLayer()
	if selectedCommand != "" {
		state.command(selectedCommand)
	}
	if selectedIndex >= 0 {
		chosen := tabs[selectedIndex]
		state.cancelMacroForCommand()
		state.menuOpen, state.windowSortMenuOpen = -1, false
		state.showInActivePane(chosen)
		state.space.Select(state.documentIndex(chosen))
	}
	if state.windowSortMenuOpen && state.menuOpen == 11 {
		state.paintWindowSortMenu(canvas, size, x+width)
	}
}

func (state *model) paintOverflowMenu(canvas *gfx.Canvas, size gfx.Size) {
	canvas.SetEscapeTarget(menuDismissID)
	canvas.PushLayer(1)
	canvas.HitWithOptions(menuDismissID, gfx.Rect{Y: menuHeight, Width: size.Width, Height: size.Height - menuHeight}, "Close hidden menus", gfx.HitOptions{Role: gfx.RoleButton})
	if canvas.Clicked(menuDismissID) {
		state.menuOverflowOpen = false
		canvas.PopLayer()
		return
	}
	canvas.PopLayer()
	width := min(float32(180), size.Width-8)
	x := size.Width - width - 4
	count := len(menuLabels) - state.menuFirstHidden
	height := float32(count)*25 + 8
	canvas.PushLayer(2)
	canvas.Border(gfx.Rect{X: x, Y: menuHeight, Width: width, Height: height}, 1, palette.border, palette.white)
	for index := state.menuFirstHidden; index < len(menuLabels); index++ {
		label := menuLabels[index]
		item := gfx.Rect{X: x + 4, Y: menuHeight + 4 + float32(index-state.menuFirstHidden)*25, Width: width - 8, Height: 25}
		id := hiddenMenuScope.At(index)
		enabled := index < len(menuCommands) && menuCommands[index][0] != ""
		color := palette.text
		if !enabled {
			color = palette.muted
		} else if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, label, gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, color, 11)
		canvas.HitWithOptions(id, item, label, gfx.HitOptions{Role: gfx.RoleMenuItem, Disabled: !enabled})
		if enabled && canvas.Clicked(id) {
			state.menuOpen, state.menuOverflowOpen = index, false
			if index == 11 {
				state.windowRecentOffset = 0
			}
		}
	}
	canvas.PopLayer()
}

func (state *model) paintWindowSortMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	width := min(float32(252), size.Width-8)
	x := parentRight
	if x+width > size.Width-4 {
		x = max(4, parentRight-300-width)
	}
	height := float32(len(windowSortCommands))*24 + 8
	y := min(menuHeight+4, size.Height-statusHeight-height-4)
	canvas.PushLayer(3)
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(windowSortPanelID, panel, "Sort windows", gfx.HitOptions{Role: gfx.RoleMenu})
	selected := -1
	for index, command := range windowSortCommands {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*24, Width: width - 8, Height: 24}
		id := windowSortScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(command.label, 11, gfx.WeightRegular, item.Width-18)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command.label, gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(windowSortCommands[selected].label)
	}
}
