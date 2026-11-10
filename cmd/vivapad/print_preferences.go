package main

import (
	"fmt"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/printing"
)

var printVariableOptions = [...]struct{ label, value string }{
	{"Full file path", "$(FULL_CURRENT_PATH)"},
	{"File name", "$(FILE_NAME)"},
	{"File directory", "$(CURRENT_DIRECTORY)"},
	{"Page", "$(CURRENT_PRINTING_PAGE)"},
	{"Short date", "$(SHORT_DATE)"},
	{"Long date", "$(LONG_DATE)"},
	{"Time", "$(TIME)"},
}

func (state *model) openPrintPreferenceFields() {
	settings := state.printSettings
	for index, value := range []string{
		settings.Header.Left, settings.Header.Middle, settings.Header.Right,
		settings.Footer.Left, settings.Footer.Middle, settings.Footer.Right,
	} {
		state.printFields[index] = text.NewEditor(value)
	}
	state.printFieldSelected, state.printVariableIndex, state.printVariableOpen = 0, 0, false
}

func (state *model) printPreferenceField(id ui.ID) bool {
	if state.preferencesPage != 3 {
		return false
	}
	for index := range state.printFields {
		if id == printBandFieldScope.At(index) {
			return true
		}
	}
	return false
}

func (state *model) updatePrintBand(index int, value string) error {
	settings := state.printSettings
	switch index {
	case 0:
		settings.Header.Left = value
	case 1:
		settings.Header.Middle = value
	case 2:
		settings.Header.Right = value
	case 3:
		settings.Footer.Left = value
	case 4:
		settings.Footer.Middle = value
	case 5:
		settings.Footer.Right = value
	default:
		return fmt.Errorf("printing: invalid band position")
	}
	if err := settings.Validate(); err != nil {
		state.notice = "Print settings not saved: " + err.Error()
		return err
	}
	state.printSettings = settings
	return state.savePreferences()
}

func (state *model) printPreferenceTextInput(id ui.ID) *app.TextInput {
	if !state.printPreferenceField(id) {
		return nil
	}
	for index, editor := range state.printFields {
		if id != printBandFieldScope.At(index) || editor == nil {
			continue
		}
		return &app.TextInput{State: app.TextStateOf(editor), OnEvent: func(event app.TextEvent) (app.TextState, error) {
			if event.Kind == app.TextCommandEvent && (event.Command == app.TextInsertNewline || event.Command == app.TextInsertLineBreak) {
				return app.TextStateOf(editor), nil
			}
			event.Apply(editor)
			if event.Kind != app.TextSetSelection {
				if _, composing := editor.Composition(); !composing {
					state.updatePrintBand(index, editor.String())
				}
			}
			return app.TextStateOf(editor), nil
		}}
	}
	return nil
}

func (state *model) adjustPrintMargin(index, delta int) {
	settings := state.printSettings
	var margin *int
	switch index {
	case 0:
		margin = &settings.MarginMM.Top
	case 1:
		margin = &settings.MarginMM.Left
	case 2:
		margin = &settings.MarginMM.Right
	case 3:
		margin = &settings.MarginMM.Bottom
	default:
		return
	}
	if *margin+delta < 0 || *margin+delta > 10000 {
		return
	}
	*margin += delta
	state.printSettings = settings
	state.savePreferences()
}

func (state *model) insertPrintVariable() {
	index := state.printFieldSelected
	if index < 0 || index >= len(state.printFields) || state.printFields[index] == nil {
		return
	}
	editor := state.printFields[index]
	before := state.printSettings
	editor.ReplaceRange(editor.Selection(), printVariableOptions[state.printVariableIndex].value)
	if err := state.updatePrintBand(index, editor.String()); err != nil && state.printSettings == before {
		editor.Undo()
	}
}

func (state *model) paintPrintPreferences(canvas *ui.Canvas, x, y, width float32) {
	smallLabel(canvas, "Print", ui.Rect{X: x, Y: y + 64, Width: width, Height: 25}, palette.text, 14)
	smallLabel(canvas, "Color options", ui.Rect{X: x, Y: y + 89, Width: width, Height: 18}, palette.muted, 11)
	for index, option := range []struct {
		label string
		mode  printing.ColorMode
	}{
		{"WYSIWYG", printing.ColorWYSIWYG},
		{"Invert", printing.ColorInvertLight},
		{"Black on white", printing.ColorBlackOnWhite},
		{"No background", printing.ColorOnWhite},
	} {
		bounds := ui.Rect{X: x + float32(index%2)*(width/2), Y: y + 109 + float32(index/2)*29, Width: width/2 - 6, Height: 25}
		id := printColorScope.At(index)
		background := palette.white
		if state.printSettings.ColorMode == option.mode || canvas.Hovered() == id {
			background = palette.hover
		}
		canvas.Border(bounds, 1, palette.border, background)
		color := palette.text
		if state.printSettings.ColorMode == option.mode {
			color = palette.blue
		}
		canvas.LabelCenter(option.label, bounds, ui.TextStyle{Size: 11, Color: color})
		canvas.HitWithOptions(id, bounds, option.label, ui.HitOptions{Role: ui.RoleRadio, Checked: state.printSettings.ColorMode == option.mode})
		if canvas.Clicked(id) {
			state.printSettings.ColorMode = option.mode
			state.savePreferences()
		}
	}
	state.preferenceCheckbox(canvas, printLineNumbersID, ui.Rect{X: x, Y: y + 169, Width: width / 2, Height: 24}, "Print line numbers", state.printSettings.LineNumbers, "Print Line Numbers")
	state.preferenceCheckbox(canvas, printFormFeedID, ui.Rect{X: x + width/2, Y: y + 169, Width: width / 2, Height: 24}, "Form feed as page break", state.printSettings.FormFeedPageBreak, "Print Form Feed")
	smallLabel(canvas, "Margins (mm)", ui.Rect{X: x, Y: y + 201, Width: width, Height: 21}, palette.muted, 11)
	values := []struct {
		label string
		value int
	}{
		{"Top", state.printSettings.MarginMM.Top}, {"Left", state.printSettings.MarginMM.Left},
		{"Right", state.printSettings.MarginMM.Right}, {"Bottom", state.printSettings.MarginMM.Bottom},
	}
	for index, item := range values {
		columnWidth := width / 2
		bounds := ui.Rect{X: x + float32(index%2)*columnWidth, Y: y + 227 + float32(index/2)*29, Width: columnWidth - 6, Height: 25}
		state.paintPrintMarginStepper(canvas, bounds, index, item.label, item.value)
	}
}

func (state *model) paintPrintMarginStepper(canvas *ui.Canvas, bounds ui.Rect, index int, label string, value int) {
	smallLabel(canvas, label, ui.Rect{X: bounds.X, Y: bounds.Y, Width: 52, Height: bounds.Height}, palette.text, 11)
	for _, control := range []struct {
		id    ui.ID
		x     float32
		glyph string
		delta int
	}{
		{printMarginDecrementScope.At(index), bounds.X + 54, "-", -1},
		{printMarginIncrementScope.At(index), bounds.X + bounds.Width - 27, "+", 1},
	} {
		button := ui.Rect{X: control.x, Y: bounds.Y, Width: 25, Height: bounds.Height}
		canvas.Border(button, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(control.glyph, button, ui.TextStyle{Size: 13, Color: palette.text})
		canvas.HitWithOptions(control.id, button, fmt.Sprintf("%s %s margin", map[bool]string{true: "Increase", false: "Decrease"}[control.delta > 0], label), ui.HitOptions{Role: ui.RoleButton})
		if canvas.Clicked(control.id) {
			state.adjustPrintMargin(index, control.delta)
		}
	}
	valueBounds := ui.Rect{X: bounds.X + 82, Y: bounds.Y, Width: bounds.Width - 112, Height: bounds.Height}
	canvas.LabelCenter(fmt.Sprint(value), valueBounds, ui.TextStyle{Size: 11, Color: palette.text})
}

func (state *model) paintPrintBandsPreferences(canvas *ui.Canvas, x, y, width float32) {
	smallLabel(canvas, "Header and Footer", ui.Rect{X: x, Y: y + 64, Width: width, Height: 25}, palette.text, 14)
	theme := ui.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 27, 11, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	for index := range state.printFields {
		column, section := index%3, index/3
		columnWidth := width / 3
		position := x + float32(column)*columnWidth
		label := []string{"Left", "Middle", "Right"}[column]
		if column == 0 {
			sectionLabel := "Header"
			if section == 1 {
				sectionLabel = "Footer"
			}
			smallLabel(canvas, sectionLabel, ui.Rect{X: x, Y: y + 90 + float32(section)*71, Width: width, Height: 19}, palette.muted, 11)
		}
		smallLabel(canvas, label, ui.Rect{X: position, Y: y + 111 + float32(section)*71, Width: columnWidth - 6, Height: 17}, palette.text, 10)
		id := printBandFieldScope.At(index)
		value := canvas.FitLine(state.printFields[index].String(), 11, ui.WeightRegular, columnWidth-36)
		components.PaintInput(canvas, theme, ui.Rect{X: position, Y: y + 130 + float32(section)*71, Width: columnWidth - 7, Height: 27}, components.InputSnapshot{
			ID: id, Label: sectionLabelForField(index) + " " + label, Value: value, PlatformEditing: canvas.Focused() == id,
		})
		if canvas.Focused() == id || canvas.Clicked(id) {
			state.printFieldSelected = index
		}
	}
	variableBounds := ui.Rect{X: x, Y: y + 239, Width: width - 89, Height: 27}
	canvas.Border(variableBounds, 1, palette.border, palette.white)
	smallLabel(canvas, "Variable: "+printVariableOptions[state.printVariableIndex].label, variableBounds.Inset(ui.All(8)), palette.text, 11)
	canvas.DrawIcon(downIcon, ui.Rect{X: variableBounds.X + variableBounds.Width - 22, Y: variableBounds.Y + 5, Width: 16, Height: 16}, palette.muted)
	canvas.HitWithOptions(printVariableButtonID, variableBounds, "Choose print variable", ui.HitOptions{Role: ui.RoleButton})
	if canvas.Clicked(printVariableButtonID) {
		state.printVariableOpen = !state.printVariableOpen
	}
	insertBounds := ui.Rect{X: x + width - 82, Y: y + 239, Width: 82, Height: 27}
	canvas.Border(insertBounds, 1, palette.border, palette.toolbar)
	canvas.LabelCenter("Insert", insertBounds, ui.TextStyle{Size: 11, Color: palette.text})
	if hitButton(canvas, printInsertVariableID, insertBounds, "Insert variable") {
		state.insertPrintVariable()
	}
	if state.printVariableOpen {
		state.paintPrintVariableMenu(canvas, x, y)
	}
}

func sectionLabelForField(index int) string {
	if index < 3 {
		return "Header"
	}
	return "Footer"
}

func (state *model) paintPrintVariableMenu(canvas *ui.Canvas, x, y float32) {
	canvas.PushLayer(5)
	panel := ui.Rect{X: x, Y: y + 63, Width: 216, Height: float32(len(printVariableOptions))*25 + 4}
	canvas.Border(panel, 1, palette.border, palette.white)
	for index, item := range printVariableOptions {
		bounds := ui.Rect{X: panel.X + 2, Y: panel.Y + 2 + float32(index)*25, Width: panel.Width - 4, Height: 25}
		id := printVariableScope.At(index)
		if canvas.Hovered() == id || state.printVariableIndex == index {
			canvas.Fill(bounds, palette.hover)
		}
		smallLabel(canvas, item.label, bounds.Inset(ui.All(7)), palette.text, 11)
		canvas.HitWithOptions(id, bounds, item.label, ui.HitOptions{Role: ui.RoleMenuItem})
		if canvas.Clicked(id) {
			state.printVariableIndex, state.printVariableOpen = index, false
		}
	}
	canvas.PopLayer()
}

func (state *model) adjustPrintFont(index int, delta int) {
	settings := state.printSettings
	font := &settings.HeaderFont
	if index == 1 {
		font = &settings.FooterFont
	}
	size := font.Points() + delta
	if size < 6 || size > 14 {
		return
	}
	font.Size = size
	state.printSettings = settings
	state.savePreferences()
}

func (state *model) paintPrintFontPreferences(canvas *ui.Canvas, x, y, width float32) {
	smallLabel(canvas, "Print Fonts", ui.Rect{X: x, Y: y + 64, Width: width, Height: 25}, palette.text, 14)
	for index, font := range []printing.BandFont{state.printSettings.HeaderFont, state.printSettings.FooterFont} {
		name := "Header"
		if index == 1 {
			name = "Footer"
		}
		baseY := y + 101 + float32(index)*83
		smallLabel(canvas, name, ui.Rect{X: x, Y: baseY, Width: width, Height: 21}, palette.muted, 11)
		smallLabel(canvas, "Size", ui.Rect{X: x, Y: baseY + 29, Width: 48, Height: 27}, palette.text, 11)
		for _, control := range []struct {
			id    ui.ID
			x     float32
			glyph string
			delta int
		}{
			{printFontSizeDecreaseScope.At(index), x + 52, "-", -1},
			{printFontSizeIncreaseScope.At(index), x + 140, "+", 1},
		} {
			bounds := ui.Rect{X: control.x, Y: baseY + 29, Width: 27, Height: 27}
			canvas.Border(bounds, 1, palette.border, palette.toolbar)
			canvas.LabelCenter(control.glyph, bounds, ui.TextStyle{Size: 13, Color: palette.text})
			label := "Increase " + name + " font size"
			if control.delta < 0 {
				label = "Decrease " + name + " font size"
			}
			canvas.HitWithOptions(control.id, bounds, label, ui.HitOptions{Role: ui.RoleButton})
			if canvas.Clicked(control.id) {
				state.adjustPrintFont(index, control.delta)
			}
		}
		canvas.LabelCenter(fmt.Sprintf("%d pt", font.Points()), ui.Rect{X: x + 80, Y: baseY + 29, Width: 58, Height: 27}, ui.TextStyle{Size: 11, Color: palette.text})
		for _, style := range []struct {
			id      ui.ID
			x       float32
			glyph   string
			label   string
			checked bool
		}{
			{printFontBoldScope.At(index), x + width - 80, "B", "Bold", font.Bold},
			{printFontItalicScope.At(index), x + width - 41, "I", "Italic", font.Italic},
		} {
			bounds := ui.Rect{X: style.x, Y: baseY + 29, Width: 31, Height: 27}
			background := palette.white
			if style.checked || canvas.Hovered() == style.id {
				background = palette.hover
			}
			canvas.Border(bounds, 1, palette.border, background)
			color := palette.text
			if style.checked {
				color = palette.blue
			}
			canvas.LabelCenter(style.glyph, bounds, ui.TextStyle{Size: 12, Color: color})
			canvas.HitWithOptions(style.id, bounds, name+" "+style.label, ui.HitOptions{Role: ui.RoleCheckbox, Checked: style.checked})
			if canvas.Clicked(style.id) {
				settings := state.printSettings
				target := &settings.HeaderFont
				if index == 1 {
					target = &settings.FooterFont
				}
				if style.label == "Bold" {
					target.Bold = !target.Bold
				} else {
					target.Italic = !target.Italic
				}
				state.printSettings = settings
				state.savePreferences()
			}
		}
	}
}
