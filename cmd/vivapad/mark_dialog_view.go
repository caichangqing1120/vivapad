package main

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

func (state *model) paintMarkDialog(canvas *ui.Canvas, size ui.Size) {
	canvas.SetEscapeTarget(markCloseID)
	canvas.PushLayer(1)
	width, height := min(float32(560), size.Width-24), float32(314)
	x := (size.Width - width) / 2
	y := max(float32(28), min(float32(72), size.Height-statusHeight-height-8))
	panel := ui.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.RoundedBorder(panel, 4, 1, palette.border, palette.white)
	canvas.HitWithOptions(markPanelID, panel, "Mark", ui.HitOptions{Role: ui.RoleGroup, SkipFocus: true})
	smallLabel(canvas, "Mark", ui.Rect{X: x + 18, Y: y + 9, Width: width - 36, Height: 28}, palette.text, 15)
	canvas.Fill(ui.Rect{X: x + 1, Y: y + 42, Width: width - 2, Height: 1}, palette.border)

	smallLabel(canvas, "Find what", ui.Rect{X: x + 18, Y: y + 54, Width: 73, Height: 28}, palette.text, 11)
	theme := ui.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 28, 12, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	components.PaintInput(canvas, theme, ui.Rect{X: x + 94, Y: y + 54, Width: width - 112, Height: 28}, components.InputSnapshot{
		ID: queryID, Label: "Find what", Value: state.query.String(), PlatformEditing: canvas.Focused() == queryID,
	})
	if state.markFocusPending {
		canvas.RequestFocus(queryID)
		state.markFocusPending = false
	}
	checkbox := func(id ui.ID, label string, checked *bool, left, top, itemWidth float32) {
		bounds := ui.Rect{X: x + left, Y: y + top, Width: itemWidth, Height: 25}
		box := ui.Rect{X: bounds.X, Y: bounds.Y + 4, Width: 15, Height: 15}
		canvas.Border(box, 1, palette.border, palette.white)
		if *checked {
			canvas.DrawIcon(checkIcon, box.Inset(ui.All(2)), palette.blue)
		}
		smallLabel(canvas, label, ui.Rect{X: bounds.X + 20, Y: bounds.Y, Width: itemWidth - 21, Height: bounds.Height}, palette.text, 11)
		canvas.HitWithOptions(id, bounds, label, ui.HitOptions{Role: ui.RoleCheckbox, Checked: *checked})
		if canvas.Clicked(id) {
			*checked = !*checked
		}
	}
	checkbox(matchCaseID, "Match case", &state.searchOptions.MatchCase, 18, 96, 116)
	checkbox(wholeWordID, "Whole word", &state.searchOptions.WholeWord, 146, 96, 116)
	checkbox(markBookmarkID, "Bookmark line", &state.markBookmarkLine, 274, 96, 135)
	checkbox(markPurgeID, "Purge", &state.markPurge, 420, 96, 94)
	checkbox(markInSelectionID, "In selection", &state.markInSelection, 18, 128, 135)
	checkbox(markWrapID, "Wrap around", &state.markWrap, 160, 128, 135)
	if state.markMode == "regex" {
		checkbox(markDotAllID, ". matches newline", &state.markDotAll, 300, 128, 190)
	}
	smallLabel(canvas, "Search mode", ui.Rect{X: x + 18, Y: y + 163, Width: 130, Height: 19}, palette.muted, 11)
	for index, mode := range []struct{ value, label string }{
		{"normal", "Normal"}, {"extended", "Extended"}, {"regex", "Regular expression"},
	} {
		bounds := ui.Rect{X: x + 18 + float32(index)*(width-36)/3, Y: y + 187, Width: (width - 36) / 3, Height: 28}
		if columnSegment(canvas, markModeScope.At(index), bounds, mode.label, state.markMode == mode.value) {
			state.markMode = mode.value
			state.searchOptions.Regex = mode.value == "regex"
		}
	}
	if state.notice != "" {
		message := canvas.FitLine(state.notice, 11, ui.WeightRegular, width-36)
		smallLabel(canvas, message, ui.Rect{X: x + 18, Y: y + 225, Width: width - 36, Height: 21}, palette.muted, 11)
	}
	for _, button := range []struct {
		id    ui.ID
		label string
		left  float32
		width float32
	}{
		{markAllID, "Mark All", 18, 96},
		{markClearID, "Clear all marks", 122, 119},
		{markCopyID, "Copy Marked Text", 249, 145},
		{markCloseID, "Close", width - 90, 72},
	} {
		bounds := ui.Rect{X: x + button.left, Y: y + 263, Width: button.width, Height: 30}
		background := palette.toolbar
		if canvas.Hovered() == button.id {
			background = palette.hover
		}
		canvas.Border(bounds, 1, palette.border, background)
		canvas.LabelCenter(button.label, bounds, ui.TextStyle{Size: 11, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			switch button.id {
			case markAllID:
				state.markAll()
			case markClearID:
				state.clearMarkDialog()
			case markCopyID:
				state.copyMarkedText()
			case markCloseID:
				state.markDialogOpen = false
			}
		}
	}
	canvas.PopLayer()
}
