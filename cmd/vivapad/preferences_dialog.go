package main

import "github.com/vivaui/viva/gfx"

func (state *model) closePreferences() {
	state.preferencesOpen, state.printVariableOpen = false, false
	state.requestEditorCommandFocus()
}

func (state *model) paintPreferencesDialog(canvas *gfx.Canvas, size gfx.Size) {
	canvas.SetEscapeTarget(preferencesCloseID)
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Preferences dialog", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width, height := min(float32(590), size.Width-32), min(float32(340), size.Height-32)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	smallLabel(canvas, "Preferences", gfx.Rect{X: x + 19, Y: y + 13, Width: width - 38, Height: 29}, palette.text, 15)
	canvas.Fill(gfx.Rect{X: x + 1, Y: y + 51, Width: width - 2, Height: 1}, palette.border)
	canvas.Fill(gfx.Rect{X: x + 146, Y: y + 52, Width: 1, Height: height - 109}, palette.border)
	for index, category := range []struct {
		id    gfx.ID
		label string
	}{{preferencesEditingID, "Editing"}, {preferencesViewID, "View"}, {preferencesPrintID, "Print"}, {preferencesPrintBandsID, "Header/Footer"}, {preferencesPrintFontsID, "Print Fonts"}, {preferencesSymbolsID, "Symbols"}} {
		bounds := gfx.Rect{X: x + 12, Y: y + 65 + float32(index)*35, Width: 124, Height: 31}
		if state.preferencesPage == index || canvas.Hovered() == category.id {
			canvas.Fill(bounds, palette.hover)
		}
		color := palette.text
		if state.preferencesPage == index {
			color = palette.blue
		}
		smallLabel(canvas, category.label, bounds.Inset(gfx.All(9)), color, 12)
		canvas.HitWithOptions(category.id, bounds, category.label, gfx.HitOptions{Role: gfx.RoleTab, Selected: state.preferencesPage == index})
		if canvas.Clicked(category.id) {
			state.preferencesPage = index
		}
	}
	contentX, contentWidth := x+165, width-183
	switch state.preferencesPage {
	case 0:
		state.paintEditingPreferences(canvas, contentX, y, contentWidth)
	case 1:
		state.paintViewPreferences(canvas, contentX, y, contentWidth)
	case 2:
		state.paintPrintPreferences(canvas, contentX, y, contentWidth)
	case 3:
		state.paintPrintBandsPreferences(canvas, contentX, y, contentWidth)
	case 4:
		state.paintPrintFontPreferences(canvas, contentX, y, contentWidth)
	case 5:
		state.paintSymbolPreferences(canvas, contentX, y, contentWidth)
	}
	canvas.Fill(gfx.Rect{X: x + 1, Y: y + height - 54, Width: width - 2, Height: 1}, palette.border)
	closeBounds := gfx.Rect{X: x + width - 103, Y: y + height - 43, Width: 82, Height: 29}
	canvas.Border(closeBounds, 1, palette.border, palette.toolbar)
	canvas.LabelCenter("Close", closeBounds, gfx.TextStyle{Size: 12, Color: palette.text})
	if hitButton(canvas, preferencesCloseID, closeBounds, "Close Preferences") {
		state.closePreferences()
	}
	canvas.PopLayer()
	canvas.PopLayer()
}

func (state *model) paintEditingPreferences(canvas *gfx.Canvas, x, y, width float32) {
	smallLabel(canvas, "Editing", gfx.Rect{X: x, Y: y + 64, Width: width, Height: 27}, palette.text, 14)
	smallLabel(canvas, "Tab width", gfx.Rect{X: x, Y: y + 91, Width: width, Height: 21}, palette.muted, 11)
	for index, value := range []int{2, 4, 8} {
		label := []string{"2", "4", "8"}[index]
		state.preferenceChoice(canvas, preferencesTabScope.At(index), gfx.Rect{X: x + float32(index)*(width/3), Y: y + 114, Width: width/3 - 5, Height: 28}, label, "Tab width: "+label, state.tabWidth == value, "Tab Width: "+label)
	}
	smallLabel(canvas, "Indent using", gfx.Rect{X: x, Y: y + 154, Width: width, Height: 21}, palette.muted, 11)
	for index, option := range []struct {
		label, value, command string
	}{{"Language", "language", "Indent: Language Default"}, {"Tabs", "tabs", "Indent: Tabs"}, {"Spaces", "spaces", "Indent: Spaces"}} {
		state.preferenceChoice(canvas, preferencesIndentScope.At(index), gfx.Rect{X: x + float32(index)*(width/3), Y: y + 177, Width: width/3 - 5, Height: 28}, option.label, "Indent: "+option.label, state.indentMode == option.value, option.command)
	}
	state.preferenceCheckbox(canvas, preferencesAutoIndentID, gfx.Rect{X: x, Y: y + 220, Width: width, Height: 27}, "Auto Indent", state.autoIndent, "Auto Indent")
	state.preferenceCheckbox(canvas, preferencesPairBracketsID, gfx.Rect{X: x, Y: y + 252, Width: width, Height: 27}, "Pair Brackets", state.autoClose, "Pair Brackets")
}

func (state *model) paintViewPreferences(canvas *gfx.Canvas, x, y, width float32) {
	smallLabel(canvas, "View", gfx.Rect{X: x, Y: y + 64, Width: width, Height: 27}, palette.text, 14)
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
		state.preferenceCheckbox(canvas, option.id, gfx.Rect{X: x, Y: y + 97 + float32(index)*33, Width: width, Height: 27}, option.label, option.checked, option.command)
	}
}

func (state *model) paintSymbolPreferences(canvas *gfx.Canvas, x, y, width float32) {
	smallLabel(canvas, "Symbols", gfx.Rect{X: x, Y: y + 64, Width: width, Height: 27}, palette.text, 14)
	smallLabel(canvas, "Non-printing style", gfx.Rect{X: x, Y: y + 98, Width: width, Height: 21}, palette.muted, 11)
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
	background := palette.white
	if checked || canvas.Hovered() == id {
		background = palette.hover
	}
	canvas.Border(bounds, 1, palette.border, background)
	color := palette.text
	if checked {
		color = palette.blue
	}
	canvas.LabelCenter(text, bounds, gfx.TextStyle{Size: 11, Color: color})
	canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleRadio, Checked: checked})
	if canvas.Clicked(id) && !checked {
		state.command(command)
	}
}

func (state *model) preferenceCheckbox(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, label string, checked bool, command string) {
	if canvas.Hovered() == id {
		canvas.Fill(bounds, palette.hover)
	}
	box := gfx.Rect{X: bounds.X + 1, Y: bounds.Y + 5, Width: 17, Height: 17}
	canvas.Border(box, 1, palette.border, palette.white)
	if checked {
		canvas.DrawIcon(checkIcon, box.Inset(gfx.All(2)), palette.blue)
	}
	smallLabel(canvas, canvas.FitLine(label, 11, gfx.WeightRegular, bounds.Width-33), gfx.Rect{X: bounds.X + 30, Y: bounds.Y, Width: bounds.Width - 32, Height: bounds.Height}, palette.text, 11)
	canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: checked})
	if canvas.Clicked(id) {
		state.command(command)
	}
}
