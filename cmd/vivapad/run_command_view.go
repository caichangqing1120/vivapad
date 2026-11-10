package main

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func (state *model) paintRunMenu(canvas *ui.Canvas, size ui.Size, origin float32) {
	count := len(state.namedRuns) + 2
	width := min(float32(290), size.Width-8)
	x := min(origin, size.Width-width-2)
	available := size.Height - menuHeight - statusHeight - 8
	rowHeight, controls, visible := float32(24), float32(0), count
	if float32(count)*rowHeight+8 > available {
		controls = 44
		visible = max(1, int((available-controls-8)/rowHeight))
	}
	state.runMenuVisible = visible
	state.runMenuOffset = min(max(0, state.runMenuOffset), max(0, count-visible))
	height := float32(visible)*rowHeight + controls + 8
	y := menuHeight
	canvas.PushLayer(2)
	canvas.PushOverlayClip(ui.Rect{Width: size.Width, Height: size.Height})
	panel := ui.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(runMenuPanelID, panel, "Run menu", ui.HitOptions{Role: ui.RoleMenu})
	if controls > 0 {
		for _, control := range []struct {
			id    ui.ID
			icon  ui.Icon
			label string
			y     float32
			step  int
		}{
			{runMenuPreviousID, upIcon, "Previous Run commands", y + 4, -1},
			{runMenuNextID, downIcon, "Next Run commands", y + height - 26, 1},
		} {
			bounds := ui.Rect{X: x + 4, Y: control.y, Width: width - 8, Height: 22}
			canvas.DrawIcon(control.icon, ui.Rect{X: bounds.X + bounds.Width/2 - 8, Y: bounds.Y + 3, Width: 16, Height: 16}, palette.muted)
			if hitButton(canvas, control.id, bounds, control.label) {
				state.runMenuOffset = min(max(0, state.runMenuOffset+control.step), count-visible)
			}
		}
	}
	rows := ui.Rect{X: x + 4, Y: y + 4 + controls/2, Width: width - 8, Height: float32(visible) * rowHeight}
	canvas.Clip(rows)
	chosen := -1
	for row := 0; row < visible; row++ {
		index := state.runMenuOffset + row
		item := ui.Rect{X: rows.X, Y: rows.Y + float32(row)*rowHeight, Width: rows.Width, Height: rowHeight}
		id, label, disabled := runOpenID, "Run...", false
		switch {
		case index == count-1:
			id, label, disabled = runNamedScope.At(index-1), "Validate shortcuts.xml", true
		case index > 0:
			id, label = runNamedScope.At(index-1), state.namedRuns[index-1].Name
		}
		color := palette.text
		if disabled {
			color = palette.muted
		} else if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, canvas.FitLine(label, 11, ui.WeightRegular, item.Width-18), ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, color, 11)
		canvas.HitWithOptions(id, item, label, ui.HitOptions{Role: ui.RoleMenuItem, Disabled: disabled})
		if !disabled && canvas.Clicked(id) {
			chosen = index
		}
	}
	canvas.Unclip()
	canvas.PopOverlayClip()
	canvas.PopLayer()
	if chosen == 0 {
		state.command("Run...")
	} else if chosen > 0 {
		state.menuOpen = -1
		if err := state.launchRunTemplate(state.namedRuns[chosen-1].Template); err != nil {
			state.notice = "Run failed: " + err.Error()
		}
	}
}

func (state *model) paintRunDialog(canvas *ui.Canvas, size ui.Size) {
	canvas.SetEscapeTarget(runDialogCancelID)
	canvas.PushLayer(3)
	canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, ui.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, ui.Rect{Width: size.Width, Height: size.Height}, "Run dialog", ui.HitOptions{Role: ui.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width, height := min(float32(480), size.Width-32), float32(210)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(ui.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	title := "Run..."
	if state.runSaveMode {
		title = "Save Run Command"
	}
	smallLabel(canvas, title, ui.Rect{X: x + 20, Y: y + 14, Width: width - 40, Height: 27}, palette.text, 15)
	theme := ui.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 30, 12, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	if state.runSaveMode {
		components.PaintInput(canvas, theme, ui.Rect{X: x + 20, Y: y + 55, Width: width - 40, Height: 30}, components.InputSnapshot{
			ID: runDialogNameID, Label: "Run command name", Value: state.runName.String(), PlatformEditing: canvas.Focused() == runDialogNameID,
		})
		label := canvas.FitLine(state.runInput.String(), 11, ui.WeightRegular, width-40)
		smallLabel(canvas, label, ui.Rect{X: x + 20, Y: y + 101, Width: width - 40, Height: 22}, palette.muted, 11)
	} else {
		components.PaintInput(canvas, theme, ui.Rect{X: x + 20, Y: y + 55, Width: width - 112, Height: 30}, components.InputSnapshot{
			ID: runDialogInputID, Label: "Program to Run", Value: state.runInput.String(), PlatformEditing: canvas.Focused() == runDialogInputID,
		})
		for _, button := range []struct {
			id    ui.ID
			icon  ui.Icon
			label string
			x     float32
		}{
			{runDialogBrowseID, openIcon, "Choose program", x + width - 86},
			{runDialogVariablesID, newAccentIcon, "Insert variable", x + width - 53},
		} {
			bounds := ui.Rect{X: button.x, Y: y + 56, Width: 27, Height: 28}
			canvas.Border(bounds, 1, palette.border, palette.toolbar)
			canvas.DrawIcon(button.icon, bounds.Inset(ui.All(5)), palette.text)
			if hitButton(canvas, button.id, bounds, button.label) {
				if button.id == runDialogBrowseID {
					state.browseRunExecutable()
				} else {
					state.runVariablesOpen = !state.runVariablesOpen
				}
			}
		}
	}
	if state.runDialogFocus {
		focus := runDialogInputID
		if state.runSaveMode {
			focus = runDialogNameID
		}
		canvas.RequestFocus(focus)
		state.runDialogFocus = false
	}
	if state.runDialogError != "" {
		smallLabel(canvas, canvas.FitLine(state.runDialogError, 11, ui.WeightRegular, width-40), ui.Rect{X: x + 20, Y: y + 129, Width: width - 40, Height: 20}, palette.blue, 11)
	}
	buttons := []struct {
		id    ui.ID
		label string
		x     float32
	}{
		{runDialogExecuteID, "Run", x + width - 278},
		{runDialogSaveID, "Save...", x + width - 188},
		{runDialogCancelID, "Cancel", x + width - 98},
	}
	if state.runSaveMode {
		buttons = buttons[1:]
		buttons[0].label, buttons[1].label = "Save", "Back"
	}
	for _, button := range buttons {
		bounds := ui.Rect{X: button.x, Y: y + height - 45, Width: 80, Height: 28}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, ui.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			switch button.id {
			case runDialogCancelID:
				state.cancelRunDialog()
			case runDialogSaveID:
				if state.runSaveMode {
					state.applyRunDialog()
				} else {
					state.runSaveMode, state.runVariablesOpen = true, false
					state.runName, state.runDialogFocus, state.runDialogError = text.NewEditor(""), true, ""
				}
			case runDialogExecuteID:
				state.applyRunDialog()
			}
		}
	}
	if state.runVariablesOpen && !state.runSaveMode {
		state.paintRunVariables(canvas, size, x, y, width)
	}
	canvas.PopLayer()
	canvas.PopLayer()
}

func (state *model) paintRunVariables(canvas *ui.Canvas, size ui.Size, x, y, width float32) {
	menuWidth := min(float32(225), size.Width-16)
	popupHeight := float32(len(runVariableNames))*22 + 8
	menuX := min(x+width-menuWidth, size.Width-menuWidth-4)
	menuY := min(y+93, size.Height-statusHeight-popupHeight-4)
	menuY = max(menuHeight, menuY)
	canvas.PushLayer(5)
	canvas.PushOverlayClip(ui.Rect{Width: size.Width, Height: size.Height})
	canvas.Border(ui.Rect{X: menuX, Y: menuY, Width: menuWidth, Height: popupHeight}, 1, palette.border, palette.white)
	for index, name := range runVariableNames {
		item := ui.Rect{X: menuX + 4, Y: menuY + 4 + float32(index)*22, Width: menuWidth - 8, Height: 22}
		id := runVariableScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, name, ui.Rect{X: item.X + 8, Y: item.Y, Width: item.Width - 16, Height: item.Height}, palette.text, 10)
		canvas.HitWithOptions(id, item, "Insert "+name, ui.HitOptions{Role: ui.RoleMenuItem})
		if canvas.Clicked(id) {
			state.runInput.Insert("$(" + name + ")")
			state.runVariablesOpen, state.runDialogFocus, state.runDialogError = false, true, ""
		}
	}
	canvas.PopOverlayClip()
	canvas.PopLayer()
}
