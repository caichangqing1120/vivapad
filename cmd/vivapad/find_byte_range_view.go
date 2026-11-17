package main

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func (state *model) paintByteRangeDialog(canvas *gfx.Canvas, size gfx.Size) {
	canvas.SetEscapeTarget(byteRangeCloseID)
	canvas.PushLayer(3)
	width, height := min(float32(470), size.Width-32), float32(270)
	x, y := size.Width-width-16, min(float32(100), size.Height-height-statusHeight-4)
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.RoundedBorder(panel, 4, 1, palette.border, palette.white)
	canvas.HitWithOptions(byteRangePanelID, panel, "Find characters in range", gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	smallLabel(canvas, "Find Characters in Range", gfx.Rect{X: x + 20, Y: y + 15, Width: width - 40, Height: 27}, palette.text, 15)
	canvas.Fill(gfx.Rect{X: x + 1, Y: y + 52, Width: width - 2, Height: 1}, palette.border)
	for index, label := range []string{"Non-ASCII", "ASCII", "Custom range"} {
		bounds := gfx.Rect{X: x + 20 + float32(index)*(width-32)/3, Y: y + 65, Width: (width - 48) / 3, Height: 29}
		id := byteRangeModeScope.At(index)
		if columnSegment(canvas, id, bounds, label, state.byteRangeMode == index) {
			state.byteRangeMode, state.byteRangeError = index, ""
			state.byteRangeFocus = index == 2
		}
	}
	start, end := "128", "255"
	if state.byteRangeMode == 1 {
		start, end = "0", "127"
	} else if state.byteRangeMode == 2 {
		start, end = state.byteRangeStart.String(), state.byteRangeEnd.String()
	}
	smallLabel(canvas, "Byte range", gfx.Rect{X: x + 20, Y: y + 103, Width: 100, Height: 20}, palette.muted, 11)
	theme := gfx.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 30, 12, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	for _, field := range []struct {
		id    gfx.ID
		label string
		value string
		x     float32
	}{{byteRangeStartID, "From", start, x + 113}, {byteRangeEndID, "To", end, x + 225}} {
		components.PaintInput(canvas, theme, gfx.Rect{X: field.x, Y: y + 103, Width: 97, Height: 30}, components.InputSnapshot{
			ID: field.id, Label: field.label, Value: field.value, Disabled: state.byteRangeMode != 2,
			PlatformEditing: state.byteRangeMode == 2 && canvas.Focused() == field.id,
		})
	}
	if state.byteRangeFocus {
		canvas.RequestFocus(byteRangeStartID)
		state.byteRangeFocus = false
	}
	smallLabel(canvas, "Direction", gfx.Rect{X: x + 20, Y: y + 148, Width: 85, Height: 22}, palette.muted, 11)
	for _, option := range []struct {
		id     gfx.ID
		label  string
		up     bool
		offset float32
	}{{byteRangeDownID, "Down", false, 112}, {byteRangeUpID, "Up", true, 219}} {
		if columnSegment(canvas, option.id, gfx.Rect{X: x + option.offset, Y: y + 146, Width: 96, Height: 27}, option.label, state.byteRangeUp == option.up) {
			state.byteRangeUp = option.up
		}
	}
	box := gfx.Rect{X: x + 20, Y: y + 187, Width: 17, Height: 17}
	canvas.Border(box, 1, palette.border, palette.white)
	if state.byteRangeWrap {
		canvas.DrawIcon(checkIcon, box.Inset(gfx.All(2)), palette.blue)
	}
	wrap := gfx.Rect{X: x + 19, Y: y + 182, Width: 180, Height: 28}
	smallLabel(canvas, "Wrap around", gfx.Rect{X: x + 48, Y: wrap.Y, Width: 145, Height: wrap.Height}, palette.text, 11)
	canvas.HitWithOptions(byteRangeWrapID, wrap, "Wrap around", gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: state.byteRangeWrap})
	if canvas.Clicked(byteRangeWrapID) {
		state.byteRangeWrap = !state.byteRangeWrap
	}
	if state.byteRangeError != "" {
		message := canvas.FitLine(state.byteRangeError, 11, gfx.WeightRegular, width-40)
		smallLabel(canvas, message, gfx.Rect{X: x + 20, Y: y + 211, Width: width - 40, Height: 18}, palette.blue, 11)
	}
	for _, button := range []struct {
		id     gfx.ID
		label  string
		offset float32
	}{{byteRangeFindID, "Find", 185}, {byteRangeCloseID, "Close", 95}} {
		bounds := gfx.Rect{X: x + width - button.offset, Y: y + height - 40, Width: 80, Height: 28}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, gfx.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			if button.id == byteRangeCloseID {
				state.byteRangeOpen = false
			} else {
				state.findByteRange()
			}
		}
	}
	canvas.PopLayer()
}
