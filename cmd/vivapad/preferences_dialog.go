package main

import "github.com/vivaui/viva/gfx"

func (state *model) closePreferences() {
	state.preferencesOpen, state.printVariableOpen = false, false
	state.resetPreferencesPending = false
	state.requestEditorCommandFocus()
}

func (state *model) paintPreferencesDialog(canvas *gfx.Canvas, size gfx.Size) {
	palette := state.palette()
	escape := preferencesCloseID
	if state.resetPreferencesPending {
		escape = cancelResetPreferencesID
	}
	canvas.SetEscapeTarget(escape)
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Preferences dialog", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width, height := min(float32(800), size.Width-32), min(float32(340), size.Height-32)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.popup)
	state.interfaceLabel(canvas, state.tr("Preferences", "偏好设置"), gfx.Rect{X: x + 19, Y: y + 13, Width: width - 38, Height: 29}, palette.text, 15)
	canvas.Fill(gfx.Rect{X: x + 1, Y: y + 51, Width: width - 2, Height: 1}, palette.border)
	canvas.Fill(gfx.Rect{X: x + 146, Y: y + 52, Width: 1, Height: height - 109}, palette.border)
	for index, category := range []struct {
		id    gfx.ID
		label string
		index int
	}{{preferencesGeneralID, state.tr("General", "常规"), preferencesGeneralPage}, {preferencesEditingID, "Editing", preferencesEditingPage}, {preferencesViewID, "View", preferencesViewPage}, {preferencesSymbolsID, "Symbols", preferencesSymbolsPage}, {preferencesPrintID, "Print", preferencesPrintPage}, {preferencesPrintBandsID, "Header/Footer", preferencesPrintBandsPage}, {preferencesPrintFontsID, "Print Fonts", preferencesPrintFontPage}, {preferencesAboutID, state.tr("About & Updates", "关于与更新"), preferencesAboutPage}} {
		bounds := gfx.Rect{X: x + 9, Y: y + 58 + float32(index)*28, Width: 129, Height: 26}
		selected := state.preferencesPage == category.index
		if selected || canvas.Hovered() == category.id {
			canvas.Fill(bounds, palette.hover)
		}
		color := palette.text
		if selected {
			color = palette.accent
		}
		label := state.preferenceText(category.label)
		state.interfaceLabel(canvas, label, bounds.Inset(gfx.Insets{Left: 8, Right: 6}), color, 12)
		canvas.HitWithOptions(category.id, bounds, label, gfx.HitOptions{Role: gfx.RoleTab, Selected: selected})
		if canvas.Clicked(category.id) {
			state.preferencesPage = category.index
			state.resetPreferencesPending = false
			state.preferencesNotice = ""
		}
	}
	contentX, contentWidth := x+165, width-183
	switch state.preferencesPage {
	case preferencesEditingPage:
		state.paintEditingPreferences(canvas, contentX, y, contentWidth)
	case preferencesViewPage:
		state.paintViewPreferences(canvas, contentX, y, contentWidth)
	case preferencesPrintPage:
		state.paintPrintPreferences(canvas, contentX, y, contentWidth)
	case preferencesPrintBandsPage:
		state.paintPrintBandsPreferences(canvas, contentX, y, contentWidth)
	case preferencesPrintFontPage:
		state.paintPrintFontPreferences(canvas, contentX, y, contentWidth)
	case preferencesSymbolsPage:
		state.paintSymbolPreferences(canvas, contentX, y, contentWidth)
	case preferencesGeneralPage:
		state.paintGeneralPreferences(canvas, contentX, y, contentWidth)
	case preferencesAboutPage:
		state.paintUpdatePreferences(canvas, contentX, y, contentWidth)
	}
	canvas.Fill(gfx.Rect{X: x + 1, Y: y + height - 54, Width: width - 2, Height: 1}, palette.border)
	closeBounds := gfx.Rect{X: x + width - 103, Y: y + height - 43, Width: 82, Height: 29}
	state.interfaceLabel(canvas, state.preferencesNotice, gfx.Rect{X: x + 17, Y: closeBounds.Y, Width: width - 133, Height: 29}, palette.muted, 11)
	if state.settingsButton(canvas, preferencesCloseID, closeBounds, state.tr("Close", "关闭"), false) {
		state.closePreferences()
	}
	canvas.PopLayer()
	canvas.PopLayer()
}

func (state *model) paintEditingPreferences(canvas *gfx.Canvas, x, y, width float32) {
	palette := state.palette()
	state.interfaceLabel(canvas, state.preferenceText("Editing"), gfx.Rect{X: x, Y: y + 64, Width: width, Height: 27}, palette.text, 14)
	state.interfaceLabel(canvas, state.preferenceText("Tab width"), gfx.Rect{X: x, Y: y + 91, Width: width, Height: 21}, palette.muted, 11)
	for index, value := range []int{2, 4, 8} {
		label := []string{"2", "4", "8"}[index]
		state.preferenceChoice(canvas, preferencesTabScope.At(index), gfx.Rect{X: x + float32(index)*(width/3), Y: y + 114, Width: width/3 - 5, Height: 28}, label, "Tab width: "+label, state.tabWidth == value, "Tab Width: "+label)
	}
	state.interfaceLabel(canvas, state.preferenceText("Indent using"), gfx.Rect{X: x, Y: y + 154, Width: width, Height: 21}, palette.muted, 11)
	for index, option := range []struct {
		label, value, command string
	}{{"Language", "language", "Indent: Language Default"}, {"Tabs", "tabs", "Indent: Tabs"}, {"Spaces", "spaces", "Indent: Spaces"}} {
		state.preferenceChoice(canvas, preferencesIndentScope.At(index), gfx.Rect{X: x + float32(index)*(width/3), Y: y + 177, Width: width/3 - 5, Height: 28}, option.label, "Indent: "+option.label, state.indentMode == option.value, option.command)
	}
	state.preferenceCheckbox(canvas, preferencesAutoIndentID, gfx.Rect{X: x, Y: y + 220, Width: width, Height: 27}, "Auto Indent", state.autoIndent, "Auto Indent")
	state.preferenceCheckbox(canvas, preferencesPairBracketsID, gfx.Rect{X: x, Y: y + 252, Width: width, Height: 27}, "Pair Brackets", state.autoClose, "Pair Brackets")
}

func (state *model) paintViewPreferences(canvas *gfx.Canvas, x, y, width float32) {
	palette := state.palette()
	state.interfaceLabel(canvas, state.preferenceText("View"), gfx.Rect{X: x, Y: y + 64, Width: width, Height: 27}, palette.text, 14)
	state.interfaceLabel(canvas, state.preferenceText("Theme"), gfx.Rect{X: x, Y: y + 89, Width: width, Height: 18}, palette.muted, 11)
	for index, option := range themeOptions {
		segment := width / float32(len(themeOptions))
		bounds := gfx.Rect{X: x + float32(index)*segment, Y: y + 108, Width: segment - 4, Height: 24}
		state.preferenceChoice(canvas, preferencesThemeScope.At(index), bounds, option.label, option.command(), state.themeChoice.Normalized() == option.choice, option.command())
	}
	for index, option := range []struct {
		id      gfx.ID
		label   string
		checked bool
		command string
	}{
		{preferencesLineNumbersID, "Line Numbers", state.showLineNumbers, "Show Line Numbers"},
		{preferencesWrapID, "Word Wrap", state.wrap, "Word Wrap"},
		{preferencesWhitespaceID, "Show Space and Tab", state.showWhitespace, "Show Space and Tab"},
		{preferencesEOLID, "Show End of Line", state.showEOL, "Show End of Line"},
		{preferencesIndentGuideID, "Show Indent Guide", state.showIndentGuides, "Show Indent Guide"},
		{preferencesZoomSyncID, "Synchronize Zoom Across Views", state.zoomSync, "Synchronize Zoom Across Views"},
	} {
		state.preferenceCheckbox(canvas, option.id, gfx.Rect{X: x, Y: y + 138 + float32(index)*24, Width: width, Height: 24}, option.label, option.checked, option.command)
	}
}

func (state *model) paintSymbolPreferences(canvas *gfx.Canvas, x, y, width float32) {
	palette := state.palette()
	state.interfaceLabel(canvas, state.preferenceText("Symbols"), gfx.Rect{X: x, Y: y + 64, Width: width, Height: 27}, palette.text, 14)
	state.interfaceLabel(canvas, state.preferenceText("Non-printing style"), gfx.Rect{X: x, Y: y + 98, Width: width, Height: 21}, palette.muted, 11)
	for index, option := range []struct {
		label, mode, command string
	}{
		{"Original", "identity", "Non-Printing Mode: Original"},
		{"Abbreviation", "abbreviation", "Non-Printing Mode: Abbreviation"},
		{"Codepoint", "codepoint", "Non-Printing Mode: Codepoint"},
	} {
		state.preferenceChoice(canvas, preferencesNPCModeScope.At(index), gfx.Rect{X: x + float32(index)*(width/3), Y: y + 125, Width: width/3 - 5, Height: 30}, option.label, option.command, state.npcMode == option.mode, option.command)
	}
	state.preferenceCheckbox(canvas, preferencesNPCIncludeID, gfx.Rect{X: x, Y: y + 184, Width: width, Height: 27}, "Include control / Unicode EOL", state.npcIncludeControls, "Include Control Characters in NPC Mode")
}

func (state *model) preferenceChoice(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, text, label string, checked bool, command string) {
	if state.paintPreferenceChoice(canvas, id, bounds, state.preferenceText(text), state.tr(label, state.preferenceText(text)), checked) {
		state.command(command)
	}
}

func (state *model) paintPreferenceChoice(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, text, label string, checked bool) bool {
	palette := state.palette()
	background := palette.popup
	if checked || canvas.Hovered() == id {
		background = palette.hover
	}
	canvas.Border(bounds, 1, palette.border, background)
	color := palette.text
	if checked {
		color = palette.accent
	}
	size := state.interfaceSize(11)
	canvas.LabelCenter(canvas.FitLine(text, size, gfx.WeightRegular, bounds.Width-6), bounds, gfx.TextStyle{Size: size, Color: color})
	canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleRadio, Checked: checked})
	return canvas.Clicked(id) && !checked
}

func (state *model) preferenceCheckbox(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, label string, checked bool, command string) {
	if state.paintPreferenceCheckbox(canvas, id, bounds, state.preferenceText(label), checked) {
		state.command(command)
	}
}

func (state *model) paintPreferenceCheckbox(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, label string, checked bool) bool {
	palette := state.palette()
	if canvas.Hovered() == id {
		canvas.Fill(bounds, palette.hover)
	}
	box := gfx.Rect{X: bounds.X + 1, Y: bounds.Y + 5, Width: 17, Height: 17}
	canvas.Border(box, 1, palette.border, palette.popup)
	if checked {
		canvas.DrawIcon(checkIcon, box.Inset(gfx.All(2)), palette.accent)
	}
	state.interfaceLabel(canvas, label, gfx.Rect{X: bounds.X + 27, Y: bounds.Y, Width: bounds.Width - 29, Height: bounds.Height}, palette.text, 11)
	canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: checked})
	return canvas.Clicked(id)
}
