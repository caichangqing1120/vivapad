package main

import (
	"strconv"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/macro"
)

func (state *model) openSaveMacroDialog() {
	if state.macroRecording {
		state.notice = "Stop recording before saving a macro"
		return
	}
	if len(state.macroLast) == 0 {
		state.notice = "No recorded macro to save"
		return
	}
	state.macroName = text.NewEditor("")
	state.macroDialogError = ""
	state.macroSaveOpen, state.macroDialogFocus = true, true
}

func (state *model) openRunMacroDialog() {
	if state.macroRecording {
		state.notice = "Stop recording before playback"
		return
	}
	if len(state.macroLast) == 0 && len(state.namedMacros) == 0 {
		state.notice = "No macro to run"
		return
	}
	state.macroRunIndex = -1
	if len(state.macroLast) == 0 {
		state.macroRunIndex = 0
	}
	state.macroCount = text.NewEditor("1")
	state.macroUntilEOF = false
	state.macroDialogError = ""
	state.macroRunOpen, state.macroDialogFocus = true, true
}

func (state *model) closeMacroDialog() {
	state.macroSaveOpen, state.macroRunOpen, state.macroSourceOpen = false, false, false
	state.macroName, state.macroCount = nil, nil
	state.macroDialogError = ""
}

func (state *model) macroSourceCount() int {
	count := len(state.namedMacros)
	if len(state.macroLast) != 0 {
		count++
	}
	return count
}

func (state *model) macroSourceAt(index int) (string, int) {
	if len(state.macroLast) != 0 {
		if index == 0 {
			return "Current recorded macro", -1
		}
		index--
	}
	return state.namedMacros[index].Name, index
}

func (state *model) applyMacroDialog() {
	if state.macroSaveOpen {
		name := state.macroName.String()
		candidate := append(append([]macro.Named(nil), state.namedMacros...), macro.Named{
			Name: name, Steps: append([]macro.Step(nil), state.macroLast...),
		})
		if err := macro.ValidateNamed(candidate); err != nil {
			state.macroDialogError = err.Error()
			return
		}
		if state.namedMacroPath != "" {
			if err := macro.SaveNamed(state.namedMacroPath, candidate); err != nil {
				state.macroDialogError = err.Error()
				return
			}
		}
		state.namedMacros = candidate
		state.closeMacroDialog()
		state.notice = "Saved macro " + name
		return
	}
	if !state.macroRunOpen {
		return
	}
	repeats := 1
	if !state.macroUntilEOF {
		value, err := strconv.Atoi(strings.TrimSpace(state.macroCount.String()))
		if err != nil || value < 1 || value > macro.MaxReplays {
			state.macroDialogError = "Enter a repeat count from 1 to 9999"
			return
		}
		repeats = value
	}
	steps := state.macroLast
	if state.macroRunIndex >= 0 {
		if state.macroRunIndex >= len(state.namedMacros) {
			state.macroDialogError = "Selected macro is unavailable"
			return
		}
		steps = state.namedMacros[state.macroRunIndex].Steps
	}
	if !state.playMacroSteps(steps, repeats, state.macroUntilEOF) {
		state.macroDialogError = state.notice
		return
	}
	state.closeMacroDialog()
}

func (state *model) paintSavedMacroMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(260), size.Width-8)
	x := parentRight
	if x+width > size.Width-4 {
		x = max(4, parentRight-218-width)
	}
	available := size.Height - menuHeight - statusHeight - 8
	rowHeight := float32(24)
	visible := max(1, len(state.namedMacros))
	scrolling := float32(visible)*rowHeight+8 > available
	controls := float32(0)
	if scrolling {
		controls = 44
		visible = max(1, int((available-controls-8)/rowHeight))
	}
	state.savedMacroVisible = visible
	state.savedMacroOffset = min(max(0, state.savedMacroOffset), max(0, len(state.namedMacros)-visible))
	height := float32(visible)*rowHeight + controls + 8
	y := max(menuHeight, min(menuHeight+4+7*27, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	panel := ui.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(savedMacroPanelID, panel, "Saved macros", ui.HitOptions{Role: ui.RoleMenu})
	if scrolling {
		for _, control := range []struct {
			id    ui.ID
			icon  ui.Icon
			label string
			y     float32
			step  int
		}{
			{savedMacroPrevID, upIcon, "Previous saved macros", y + 4, -1},
			{savedMacroNextID, downIcon, "Next saved macros", y + height - 26, 1},
		} {
			bounds := ui.Rect{X: x + 4, Y: control.y, Width: width - 8, Height: 22}
			canvas.DrawIcon(control.icon, ui.Rect{X: bounds.X + bounds.Width/2 - 8, Y: bounds.Y + 3, Width: 16, Height: 16}, palette.muted)
			canvas.HitWithOptions(control.id, bounds, control.label, ui.HitOptions{Role: ui.RoleMenuItem})
			if canvas.Clicked(control.id) {
				state.savedMacroOffset = min(max(0, state.savedMacroOffset+control.step), len(state.namedMacros)-visible)
			}
		}
	}
	selected := -1
	if len(state.namedMacros) == 0 {
		smallLabel(canvas, "No saved macros", ui.Rect{X: x + 12, Y: y + 4, Width: width - 24, Height: 24}, palette.muted, 11)
	}
	for row := 0; row < visible && row+state.savedMacroOffset < len(state.namedMacros); row++ {
		index := state.savedMacroOffset + row
		name := state.namedMacros[index].Name
		item := ui.Rect{X: x + 4, Y: y + 4 + controls/2 + float32(row)*rowHeight, Width: width - 8, Height: rowHeight}
		id := savedMacroScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(name, 11, ui.WeightRegular, item.Width-18)
		smallLabel(canvas, label, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, name, ui.HitOptions{Role: ui.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.menuOpen, state.savedMacroMenuOpen = -1, false
		state.playMacroSteps(state.namedMacros[selected].Steps, 1, false)
	}
}

func (state *model) paintMacroDialog(canvas *ui.Canvas, size ui.Size) {
	canvas.SetEscapeTarget(macroCancelID)
	canvas.PushLayer(3)
	canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, ui.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, ui.Rect{Width: size.Width, Height: size.Height}, "Macro dialog", ui.HitOptions{Role: ui.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width := min(float32(420), size.Width-32)
	height := float32(170)
	if state.macroRunOpen {
		height = 260
	}
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(ui.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	theme := ui.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 30, 12, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	if state.macroSaveOpen {
		smallLabel(canvas, "Save Current Recorded Macro", ui.Rect{X: x + 20, Y: y + 14, Width: width - 40, Height: 26}, palette.text, 15)
		components.PaintInput(canvas, theme, ui.Rect{X: x + 20, Y: y + 50, Width: width - 40, Height: 30}, components.InputSnapshot{
			ID: macroNameID, Label: "Macro name", Value: state.macroName.String(), PlatformEditing: canvas.Focused() == macroNameID,
		})
	} else {
		smallLabel(canvas, "Run a Macro Multiple Times", ui.Rect{X: x + 20, Y: y + 14, Width: width - 40, Height: 26}, palette.text, 15)
		sourceName := "Current recorded macro"
		if state.macroRunIndex >= 0 && state.macroRunIndex < len(state.namedMacros) {
			sourceName = state.namedMacros[state.macroRunIndex].Name
		}
		source := ui.Rect{X: x + 20, Y: y + 46, Width: width - 40, Height: 30}
		canvas.Border(source, 1, palette.border, palette.white)
		label := canvas.FitLine(sourceName, 12, ui.WeightRegular, source.Width-48)
		smallLabel(canvas, label, ui.Rect{X: source.X + 10, Y: source.Y, Width: source.Width - 42, Height: 30}, palette.text, 12)
		canvas.DrawIcon(downIcon, ui.Rect{X: source.X + source.Width - 27, Y: source.Y + 7, Width: 16, Height: 16}, palette.muted)
		canvas.HitWithOptions(macroSourceID, source, "Macro to run", ui.HitOptions{Role: ui.RoleButton, Expanded: state.macroSourceOpen})
		if canvas.Clicked(macroSourceID) {
			state.macroSourceOpen = !state.macroSourceOpen
			state.macroSourceOffset = 0
		}
		state.macroRadio(canvas, macroCountModeID, ui.Rect{X: x + 20, Y: y + 101, Width: 70, Height: 26}, "Run", !state.macroUntilEOF)
		components.PaintInput(canvas, theme, ui.Rect{X: x + 99, Y: y + 98, Width: 85, Height: 30}, components.InputSnapshot{
			ID: macroCountID, Label: "Repeat count", Value: state.macroCount.String(), PlatformEditing: canvas.Focused() == macroCountID,
		})
		smallLabel(canvas, "times", ui.Rect{X: x + 193, Y: y + 101, Width: 70, Height: 26}, palette.text, 12)
		state.macroRadio(canvas, macroEOFModeID, ui.Rect{X: x + 20, Y: y + 142, Width: width - 40, Height: 26}, "Run until end of file", state.macroUntilEOF)
	}
	if state.macroDialogFocus {
		focus := macroNameID
		if state.macroRunOpen {
			focus = macroCountID
		}
		canvas.RequestFocus(focus)
		state.macroDialogFocus = false
	}
	if state.macroDialogError != "" {
		errorY := y + 91
		if state.macroRunOpen {
			errorY = y + 181
		}
		message := canvas.FitLine(state.macroDialogError, 11, ui.WeightRegular, width-40)
		smallLabel(canvas, message, ui.Rect{X: x + 20, Y: errorY, Width: width - 40, Height: 21}, palette.blue, 11)
	}
	buttonY := y + height - 48
	for _, button := range []struct {
		id    ui.ID
		label string
		x     float32
	}{
		{macroCancelID, "Cancel", x + width - 190},
		{macroRunID, "Run macro", x + width - 100},
	} {
		if state.macroSaveOpen && button.id == macroRunID {
			button.id, button.label = macroSaveID, "Save macro"
		}
		bounds := ui.Rect{X: button.x, Y: buttonY, Width: 80, Height: 30}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, ui.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			if button.id == macroCancelID {
				state.closeMacroDialog()
			} else {
				state.applyMacroDialog()
			}
		}
	}
	if state.macroRunOpen && state.macroSourceOpen {
		state.paintMacroSources(canvas, sourceRect(x, y, width), size.Height)
	}
	canvas.PopLayer()
	canvas.PopLayer()
}

func sourceRect(x, y, width float32) ui.Rect {
	return ui.Rect{X: x + 20, Y: y + 78, Width: width - 40}
}

func (state *model) macroRadio(canvas *ui.Canvas, id ui.ID, bounds ui.Rect, label string, checked bool) {
	box := ui.Rect{X: bounds.X, Y: bounds.Y + 4, Width: 16, Height: 16}
	canvas.RoundedBorder(box, 8, 1, palette.border, palette.white)
	if checked {
		canvas.RoundedBorder(box.Inset(ui.All(4)), 4, 0, palette.blue, palette.blue)
	}
	smallLabel(canvas, label, ui.Rect{X: bounds.X + 24, Y: bounds.Y, Width: bounds.Width - 24, Height: bounds.Height}, palette.text, 12)
	canvas.HitWithOptions(id, bounds, label, ui.HitOptions{Role: ui.RoleRadio, Checked: checked})
	if canvas.Clicked(id) {
		state.macroUntilEOF = id == macroEOFModeID
	}
}

func (state *model) paintMacroSources(canvas *ui.Canvas, origin ui.Rect, viewportHeight float32) {
	count := state.macroSourceCount()
	visible := min(6, count)
	scrolling := count > visible
	controls := float32(0)
	if scrolling {
		controls = 44
	}
	state.macroSourceVisible = visible
	state.macroSourceOffset = min(max(0, state.macroSourceOffset), count-visible)
	height := float32(visible)*24 + controls + 8
	y := min(origin.Y, viewportHeight-statusHeight-height-4)
	canvas.PushLayer(5)
	canvas.Border(ui.Rect{X: origin.X, Y: y, Width: origin.Width, Height: height}, 1, palette.border, palette.white)
	if scrolling {
		for _, control := range []struct {
			id    ui.ID
			icon  ui.Icon
			label string
			y     float32
			step  int
		}{
			{macroSourcePrevID, upIcon, "Previous macros", y + 4, -1},
			{macroSourceNextID, downIcon, "Next macros", y + height - 26, 1},
		} {
			bounds := ui.Rect{X: origin.X + 4, Y: control.y, Width: origin.Width - 8, Height: 22}
			canvas.DrawIcon(control.icon, ui.Rect{X: bounds.X + bounds.Width/2 - 8, Y: bounds.Y + 3, Width: 16, Height: 16}, palette.muted)
			canvas.HitWithOptions(control.id, bounds, control.label, ui.HitOptions{Role: ui.RoleMenuItem})
			if canvas.Clicked(control.id) {
				state.macroSourceOffset = min(max(0, state.macroSourceOffset+control.step), count-visible)
			}
		}
	}
	for row := range visible {
		index := state.macroSourceOffset + row
		name, selectedIndex := state.macroSourceAt(index)
		item := ui.Rect{X: origin.X + 4, Y: y + 4 + controls/2 + float32(row)*24, Width: origin.Width - 8, Height: 24}
		id := macroSourceScope.At(index)
		label := canvas.FitLine(name, 11, ui.WeightRegular, item.Width-18)
		smallLabel(canvas, label, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, name, ui.HitOptions{Role: ui.RoleMenuItem, Checked: state.macroRunIndex == selectedIndex})
		if canvas.Clicked(id) {
			state.macroRunIndex = selectedIndex
			state.macroSourceOpen = false
		}
	}
	canvas.PopLayer()
}
