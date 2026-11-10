package main

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func (state *model) paintColumnDialog(canvas *ui.Canvas, size ui.Size) {
	canvas.SetEscapeTarget(columnCancelID)
	canvas.PushLayer(3)
	canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, ui.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, ui.Rect{Width: size.Width, Height: size.Height}, "Column Editor dialog", ui.HitOptions{Role: ui.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width, height := min(float32(520), size.Width-32), min(float32(340), size.Height-32)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(ui.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	smallLabel(canvas, "Column / Multi-Selection Editor", ui.Rect{X: x + 20, Y: y + 13, Width: width - 40, Height: 27}, palette.text, 15)
	for _, mode := range []struct {
		id     ui.ID
		label  string
		number bool
		offset float32
	}{
		{columnTextModeID, "Text", false, 20},
		{columnNumberModeID, "Number", true, 126},
	} {
		if columnSegment(canvas, mode.id, ui.Rect{X: x + mode.offset, Y: y + 51, Width: 98, Height: 28}, mode.label, state.columnNumber == mode.number) {
			state.columnNumber, state.columnError, state.columnFocus = mode.number, "", true
		}
	}
	canvas.Fill(ui.Rect{X: x + 1, Y: y + 89, Width: width - 2, Height: 1}, palette.border)
	theme := ui.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 29, 12, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	if state.columnNumber {
		state.paintColumnNumbers(canvas, theme, x, y, width)
	} else {
		smallLabel(canvas, "Text to insert", ui.Rect{X: x + 20, Y: y + 103, Width: width - 40, Height: 21}, palette.muted, 11)
		components.PaintInput(canvas, theme, ui.Rect{X: x + 20, Y: y + 126, Width: width - 40, Height: 29}, components.InputSnapshot{
			ID: columnTextID, Label: "Text to insert", Value: state.columnText.String(), PlatformEditing: canvas.Focused() == columnTextID,
		})
	}
	if state.columnFocus {
		focus := columnTextID
		if state.columnNumber {
			focus = columnInitialID
		}
		canvas.RequestFocus(focus)
		state.columnFocus = false
	}
	if state.columnError != "" {
		available := width - 40
		if state.columnNumber && state.columnBase == 16 {
			available -= 155
		}
		message := canvas.FitLine(state.columnError, 11, ui.WeightRegular, available)
		smallLabel(canvas, message, ui.Rect{X: x + 20, Y: y + height - 68, Width: available, Height: 20}, palette.blue, 11)
	}
	canvas.Fill(ui.Rect{X: x + 1, Y: y + height - 49, Width: width - 2, Height: 1}, palette.border)
	for _, button := range []struct {
		id     ui.ID
		label  string
		offset float32
	}{{columnApplyID, "Insert", 190}, {columnCancelID, "Cancel", 98}} {
		bounds := ui.Rect{X: x + width - button.offset, Y: y + height - 39, Width: 80, Height: 28}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, ui.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			if button.id == columnCancelID {
				state.columnDialogOpen = false
			} else {
				state.applyColumnDialog()
			}
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}

func (state *model) paintColumnNumbers(canvas *ui.Canvas, theme ui.Theme, x, y, width float32) {
	fieldWidth := (width - 56) / 3
	for index, field := range []struct {
		id    ui.ID
		label string
		value string
	}{{columnInitialID, "Initial number", state.columnInitial.String()}, {columnIncrementID, "Increase by", state.columnIncrement.String()}, {columnRepeatID, "Repeat", state.columnRepeat.String()}} {
		fieldX := x + 20 + float32(index)*(fieldWidth+8)
		smallLabel(canvas, field.label, ui.Rect{X: fieldX, Y: y + 101, Width: fieldWidth, Height: 20}, palette.muted, 11)
		components.PaintInput(canvas, theme, ui.Rect{X: fieldX, Y: y + 123, Width: fieldWidth, Height: 29}, components.InputSnapshot{
			ID: field.id, Label: field.label, Value: field.value, PlatformEditing: canvas.Focused() == field.id,
		})
	}
	smallLabel(canvas, "Format", ui.Rect{X: x + 20, Y: y + 165, Width: width - 40, Height: 19}, palette.muted, 11)
	for index, option := range []struct {
		label string
		base  int
	}{{"Dec", 10}, {"Hex", 16}, {"Oct", 8}, {"Bin", 2}} {
		bounds := ui.Rect{X: x + 20 + float32(index)*(width-34)/4, Y: y + 186, Width: (width - 50) / 4, Height: 27}
		if columnSegment(canvas, columnBaseScope.At(index), bounds, option.label, state.columnBase == option.base) {
			state.changeColumnBase(option.base)
		}
	}
	smallLabel(canvas, "Leading", ui.Rect{X: x + 20, Y: y + 223, Width: width - 40, Height: 19}, palette.muted, 11)
	for index, label := range []string{"None", "Zeros", "Spaces"} {
		bounds := ui.Rect{X: x + 20 + float32(index)*(width-32)/3, Y: y + 244, Width: (width - 48) / 3, Height: 27}
		if columnSegment(canvas, columnLeadingScope.At(index), bounds, label, state.columnLeading == document.ColumnLeading(index)) {
			state.columnLeading = document.ColumnLeading(index)
		}
	}
	if state.columnBase == 16 {
		bounds := ui.Rect{X: x + width - 171, Y: y + 270, Width: 151, Height: 22}
		box := ui.Rect{X: bounds.X, Y: bounds.Y + 3, Width: 16, Height: 16}
		canvas.Border(box, 1, palette.border, palette.white)
		if state.columnUppercase {
			canvas.DrawIcon(checkIcon, box.Inset(ui.All(2)), palette.blue)
		}
		smallLabel(canvas, "Uppercase A-F", ui.Rect{X: bounds.X + 24, Y: bounds.Y, Width: 127, Height: 22}, palette.text, 11)
		canvas.HitWithOptions(columnUppercaseID, bounds, "Uppercase A-F", ui.HitOptions{Role: ui.RoleCheckbox, Checked: state.columnUppercase})
		if canvas.Clicked(columnUppercaseID) {
			state.columnUppercase = !state.columnUppercase
		}
	}
}

func columnSegment(canvas *ui.Canvas, id ui.ID, bounds ui.Rect, label string, checked bool) bool {
	background := palette.white
	if checked || canvas.Hovered() == id {
		background = palette.hover
	}
	canvas.Border(bounds, 1, palette.border, background)
	color := palette.text
	if checked {
		color = palette.blue
	}
	canvas.LabelCenter(label, bounds, ui.TextStyle{Size: 11, Color: color})
	canvas.HitWithOptions(id, bounds, label, ui.HitOptions{Role: ui.RoleRadio, Checked: checked})
	return canvas.Clicked(id) && !checked
}
