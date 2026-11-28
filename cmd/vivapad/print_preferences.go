package main

import (
	"fmt"
	"slices"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
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

func (state *model) printPreferenceField(id gfx.ID) bool {
	if state.preferencesPage != preferencesPrintBandsPage {
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

func (state *model) printPreferenceTextInput(id gfx.ID) *app.TextInput {
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

func (state *model) paintPrintPreferences(canvas *gfx.Canvas, body gfx.Rect, height float32) {
	labels := state.settingsTranslations([]string{"WYSIWYG", "Invert", "Black on white", "No background"})
	modes := []printing.ColorMode{printing.ColorWYSIWYG, printing.ColorInvertLight, printing.ColorBlackOnWhite, printing.ColorOnWhite}
	title := state.preferenceText("Color options")
	extra := state.settingsSegmentExtra(canvas, body, title, labels)
	card := state.settingsCard(canvas, body, height*3+extra)
	colors, title := state.settingsSegmentRow(canvas, card, height, extra, title)
	if index := state.settingsSegments(canvas, colors, title, printColorScope,
		labels, labels, slices.Index(modes, state.printSettings.ColorMode)); index >= 0 {
		state.printSettings.ColorMode = modes[index]
		state.savePreferences()
	}
	state.preferenceToggle(canvas, printLineNumbersID, card.row(canvas, height+extra, height), "Print line numbers", state.printSettings.LineNumbers, "Print Line Numbers")
	state.preferenceToggle(canvas, printFormFeedID, card.row(canvas, height*2+extra, height), "Form feed as page break", state.printSettings.FormFeedPageBreak, "Print Form Feed")
	body.Y += card.bounds.Height + settingsCardGap
	card = state.settingsCard(canvas, body, height*3)
	state.settingsLabel(canvas, state.preferenceText("Margins (mm)"), card.row(canvas, 0, height), state.palette().text, 13, gfx.WeightSemibold)
	values := []struct {
		label string
		value int
	}{
		{"Top", state.printSettings.MarginMM.Top}, {"Left", state.printSettings.MarginMM.Left},
		{"Right", state.printSettings.MarginMM.Right}, {"Bottom", state.printSettings.MarginMM.Bottom},
	}
	for line := range 2 {
		row := card.row(canvas, float32(line+1)*height, height)
		columnWidth := (row.Width - 20) / 2
		for column := range 2 {
			index := line*2 + column
			item := values[index]
			bounds := gfx.Rect{X: row.X + float32(column)*(columnWidth+20), Y: row.Y, Width: columnWidth, Height: height}
			control := settingsControl(bounds, 108, 26)
			name := state.preferenceText(item.label)
			state.settingsRowLabel(canvas, name, bounds, control)
			delta := state.settingsStepper(canvas, control, printMarginDecrementScope.At(index), printMarginIncrementScope.At(index), fmt.Sprint(item.value),
				state.tr("Decrease "+item.label+" margin", "减小"+name+"页边距"), state.tr("Increase "+item.label+" margin", "增大"+name+"页边距"))
			if delta != 0 {
				state.adjustPrintMargin(index, delta)
			}
		}
	}
}

func (state *model) settingsStepper(canvas *gfx.Canvas, bounds gfx.Rect, decrease, increase gfx.ID, value, decreaseLabel, increaseLabel string) int {
	palette := state.palette()
	minus := gfx.Rect{X: bounds.X, Y: bounds.Y, Width: 26, Height: bounds.Height}
	plus := settingsControl(bounds, 26, bounds.Height)
	valueBounds := bounds.Inset(gfx.Insets{Left: 26, Right: 26})
	canvas.LabelCenter(value, valueBounds, gfx.TextStyle{Size: state.interfaceSize(11), Color: palette.text})
	canvas.HoverHint(page, minus, decreaseLabel, decrease)
	canvas.HoverHint(page, plus, increaseLabel, increase)
	minusClicked := state.settingsButtonWithLabel(canvas, decrease, minus, "−", decreaseLabel, false)
	plusClicked := state.settingsButtonWithLabel(canvas, increase, plus, "+", increaseLabel, false)
	if minusClicked {
		return -1
	}
	if plusClicked {
		return 1
	}
	return 0
}

func (state *model) paintPrintBandsPreferences(canvas *gfx.Canvas, body gfx.Rect, height float32) {
	palette := state.palette()
	theme := state.uiTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 28, state.interfaceSize(11), 6
	theme.Border, theme.Ring = palette.border, palette.accent
	sectionHeight := height + 48
	for section, name := range []string{"Header", "Footer"} {
		card := state.settingsCard(canvas, body, sectionHeight)
		heading := card.row(canvas, 0, height-4)
		state.settingsLabel(canvas, state.preferenceText(name), heading, palette.text, 13, gfx.WeightSemibold)
		columnWidth := (heading.Width - 16) / 3
		for column, label := range []string{"Left", "Middle", "Right"} {
			index := section*3 + column
			position := heading.X + float32(column)*(columnWidth+8)
			state.preferenceLabel(canvas, label, gfx.Rect{X: position, Y: body.Y + height - 4, Width: columnWidth, Height: 18}, palette.muted, 11)
			id := printBandFieldScope.At(index)
			value := canvas.FitLine(state.printFields[index].String(), state.interfaceSize(11), gfx.WeightRegular, columnWidth-24)
			components.PaintInput(canvas, theme, gfx.Rect{X: position, Y: body.Y + height + 15, Width: columnWidth, Height: 28}, components.InputSnapshot{
				ID: id, Label: state.preferenceText(name) + " " + state.preferenceText(label), Value: value, PlatformEditing: canvas.Focused() == id,
			})
			if canvas.Focused() == id || canvas.Clicked(id) {
				state.printFieldSelected = index
			}
		}
		body.Y += sectionHeight + settingsCardGap
	}
	card := state.settingsCard(canvas, body, height)
	row := card.row(canvas, 0, height)
	insert := state.preferenceText("Insert")
	insertBounds := settingsControl(row, state.settingsButtonWidth(canvas, insert)+8, 27)
	variableRow := row
	variableRow.Width -= insertBounds.Width + 6
	variableBounds := settingsControl(variableRow, 170, 27)
	state.settingsRowLabel(canvas, state.tr("Print variable", "打印变量"), row, variableBounds)
	border := palette.border
	if canvas.Hovered() == printVariableButtonID || canvas.FocusVisible() == printVariableButtonID || state.printVariableOpen {
		border = palette.accent
	}
	canvas.RoundedBorder(variableBounds, 7, 1, border, palette.inputBackground)
	state.interfaceLabel(canvas, state.preferenceText(printVariableOptions[state.printVariableIndex].label), variableBounds.Inset(gfx.Insets{Left: 9, Right: 25}), palette.text, 11)
	canvas.DrawIcon(downIcon, settingsControl(variableBounds.Inset(gfx.Insets{Right: 7}), 14, 14), palette.muted)
	if hitButton(canvas, printVariableButtonID, variableBounds, state.tr("Choose print variable", "选择打印变量")) {
		state.printVariableOpen = !state.printVariableOpen
	}
	if state.settingsButtonWithLabel(canvas, printInsertVariableID, insertBounds, insert, state.tr("Insert variable", "插入变量"), false) {
		state.insertPrintVariable()
	}
	if state.printVariableOpen {
		state.paintPrintVariableMenu(canvas, variableBounds)
	}
}

func (state *model) paintPrintVariableMenu(canvas *gfx.Canvas, anchor gfx.Rect) {
	palette := state.palette()
	canvas.PushLayer(5)
	height := float32(len(printVariableOptions))*25 + 8
	panel := gfx.Rect{X: anchor.X, Y: anchor.Y - height - 4, Width: anchor.Width, Height: height}
	canvas.Elevate(panel, 9, gfx.ShadowMD)
	canvas.RoundedBorder(panel, 9, 1, palette.border, palette.popup)
	for index, item := range printVariableOptions {
		bounds := gfx.Rect{X: panel.X + 4, Y: panel.Y + 4 + float32(index)*25, Width: panel.Width - 8, Height: 25}
		id := printVariableScope.At(index)
		if canvas.Hovered() == id || state.printVariableIndex == index {
			canvas.Rounded(bounds, 5, palette.selectedRow)
		}
		if canvas.FocusVisible() == id {
			canvas.RoundedBorder(bounds, 5, 1, palette.accent, gfx.Color{})
		}
		label := state.preferenceText(item.label)
		state.interfaceLabel(canvas, label, bounds.Inset(gfx.Insets{Left: 7, Right: 7}), palette.text, 11)
		canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleMenuItem})
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

func (state *model) paintPrintFontPreferences(canvas *gfx.Canvas, body gfx.Rect, height float32) {
	for index, font := range []printing.BandFont{state.printSettings.HeaderFont, state.printSettings.FooterFont} {
		name := []string{"Header", "Footer"}[index]
		localized := state.preferenceText(name)
		card := state.settingsCard(canvas, body, height*3)
		row := card.row(canvas, 0, height)
		control := settingsControl(row, 108, 26)
		state.settingsRowLabel(canvas, state.tr(name+" font size", localized+"字号"), row, control)
		delta := state.settingsStepper(canvas, control, printFontSizeDecreaseScope.At(index), printFontSizeIncreaseScope.At(index),
			fmt.Sprintf(state.tr("%d pt", "%d 磅"), font.Points()), state.tr("Decrease "+name+" font size", "减小"+localized+"字号"), state.tr("Increase "+name+" font size", "增大"+localized+"字号"))
		if delta != 0 {
			state.adjustPrintFont(index, delta)
		}
		settings := state.printSettings
		target := &settings.HeaderFont
		if index == 1 {
			target = &settings.FooterFont
		}
		if state.paintPreferenceToggle(canvas, printFontBoldScope.At(index), card.row(canvas, height, height), state.tr("Bold", "粗体"), state.tr(name+" Bold", localized+"粗体"), target.Bold) {
			target.Bold = !target.Bold
		}
		if state.paintPreferenceToggle(canvas, printFontItalicScope.At(index), card.row(canvas, height*2, height), state.tr("Italic", "斜体"), state.tr(name+" Italic", localized+"斜体"), target.Italic) {
			target.Italic = !target.Italic
		}
		if settings != state.printSettings {
			state.printSettings = settings
			state.savePreferences()
		}
		body.Y += height*3 + settingsCardGap
	}
}
