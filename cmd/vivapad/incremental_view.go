package main

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

func (state *model) incrementalHeight(width float32) float32 {
	if width < 900 {
		return 60
	}
	return 36
}

func (state *model) paintIncrementalSearch(canvas *ui.Canvas, width, y float32) {
	height := state.incrementalHeight(width)
	canvas.Fill(ui.Rect{Y: y, Width: width, Height: height}, palette.chrome)
	canvas.Fill(ui.Rect{Y: y, Width: width, Height: 1}, palette.border)
	closeBounds := ui.Rect{X: 8, Y: y + 7, Width: 22, Height: 22}
	canvas.DrawIcon(closeIcon, closeBounds.Inset(ui.All(4)), palette.muted)
	if hitButton(canvas, incrementCloseID, closeBounds, "Close incremental search") {
		state.closeIncrementalSearch()
		state.incrementReturnFocusRequested = true
		state.incrementReturnFocusArmed = false
	}
	smallLabel(canvas, "Find:", ui.Rect{X: 35, Y: y + 5, Width: 30, Height: 26}, palette.text, 11)
	fieldWidth := min(float32(330), max(160, width*0.26))
	if width < 900 {
		fieldWidth = min(280, max(120, width-230))
	}
	theme := ui.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 25, 11, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	theme.Background, theme.Foreground = palette.white, palette.text
	if state.incrementStatus == "Phrase not found" {
		theme.Background = ui.RGB(255, 234, 237)
	}
	components.PaintInput(canvas, theme, ui.Rect{X: 66, Y: y + 5, Width: fieldWidth, Height: 25}, components.InputSnapshot{
		ID: incrementQueryID, Label: "Incremental find", Value: state.incrementQuery.String(), PlatformEditing: canvas.Focused() == incrementQueryID,
	})
	previous := ui.Rect{X: 72 + fieldWidth, Y: y + 6, Width: 23, Height: 23}
	next := ui.Rect{X: 99 + fieldWidth, Y: y + 6, Width: 23, Height: 23}
	for _, button := range []struct {
		id     ui.ID
		bounds ui.Rect
		icon   ui.Icon
		label  string
	}{
		{incrementPreviousID, previous, leftIcon, "Previous incremental match"},
		{incrementNextID, next, rightIcon, "Next incremental match"},
	} {
		background := palette.toolbar
		if canvas.Hovered() == button.id {
			background = palette.hover
		}
		canvas.Border(button.bounds, 1, palette.border, background)
		canvas.DrawIcon(button.icon, button.bounds.Inset(ui.All(4)), palette.text)
		if hitButton(canvas, button.id, button.bounds, button.label) {
			state.findIncremental(button.id == incrementPreviousID, true)
		}
	}
	toggleY, toggleX := y+6, next.X+34
	if width < 900 {
		toggleY, toggleX = y+35, 12
	}
	toggles := []struct {
		id      ui.ID
		label   string
		checked *bool
		width   float32
	}{
		{incrementMatchCaseID, "Match case", &state.incrementMatchCase, 90},
		{incrementHighlightID, "Highlight all", &state.incrementHighlight, 104},
		{incrementCountID, "Count", &state.incrementCount, 70},
	}
	for _, option := range toggles {
		bounds := ui.Rect{X: toggleX, Y: toggleY, Width: option.width, Height: 22}
		box := ui.Rect{X: toggleX, Y: toggleY + 4, Width: 14, Height: 14}
		canvas.Border(box, 1, palette.border, palette.white)
		if *option.checked {
			canvas.Fill(box.Inset(ui.All(3)), palette.blue)
		}
		smallLabel(canvas, option.label, ui.Rect{X: toggleX + 19, Y: toggleY, Width: option.width - 20, Height: 22}, palette.text, 11)
		canvas.HitWithOptions(option.id, bounds, option.label, ui.HitOptions{Role: ui.RoleCheckbox, Checked: *option.checked})
		if canvas.Clicked(option.id) {
			*option.checked = !*option.checked
			switch option.id {
			case incrementMatchCaseID:
				selection := state.space.Current().Editor.Selection()
				state.findIncremental(false, false)
				if state.incrementStatus == "Phrase not found" {
					state.space.Current().Editor.MoveTo(selection.Start, false)
				}
			case incrementHighlightID:
				state.refreshIncrementMatches()
			case incrementCountID:
				state.refreshIncrementMatches()
				if state.incrementStatus != "Phrase not found" {
					state.incrementStatus = state.incrementCountStatus(state.space.Current())
				}
			}
		}
		toggleX += option.width + 8
	}
	status := ui.Rect{X: toggleX + 5, Y: toggleY, Width: max(20, width-toggleX-13), Height: 22}
	color := palette.muted
	if state.incrementStatus == "Phrase not found" {
		color = palette.blue
	}
	smallLabel(canvas, canvas.FitLine(state.incrementStatus, 11, ui.WeightRegular, status.Width), status, color, 11)
	if state.incrementOpen && state.incrementFocusPending {
		canvas.RequestFocus(incrementQueryID)
		state.incrementFocusPending = false
	}
}
