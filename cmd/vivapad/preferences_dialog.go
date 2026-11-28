package main

import (
	"slices"

	"github.com/vivaui/viva/gfx"
)

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
	viewport := gfx.Rect{Width: size.Width, Height: size.Height}
	canvas.Fill(viewport, gfx.RGBA(0, 0, 0, 0.30))
	canvas.HitWithOptions(dialogBackdropID, viewport, "Preferences dialog", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width, height := min(float32(760), max(0, size.Width-32)), min(float32(520), max(0, size.Height-48))
	x, y := (size.Width-width)/2, (size.Height-height)/2
	shell := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Elevate(shell, 12, gfx.ShadowLG)
	canvas.RoundedBorder(shell, 12, 1, palette.border, palette.editorBackground)
	sidebar := min(float32(180), width*.27)
	canvas.Clip(gfx.Rect{X: x + 1, Y: y + 1, Width: sidebar - 1, Height: height - 2})
	canvas.Rounded(shell.Inset(gfx.All(1)), 11, palette.tabs)
	canvas.Unclip()
	canvas.Fill(gfx.Rect{X: x + sidebar, Y: y + 1, Width: 1, Height: height - 2}, palette.border)
	state.settingsLabel(canvas, state.tr("Preferences", "偏好设置"), gfx.Rect{X: x + 20, Y: y + 14, Width: sidebar - 36, Height: 30}, palette.text, 16, gfx.WeightSemibold)

	categories := [...]struct {
		id                gfx.ID
		page              int
		title, detail, zh string
	}{
		{preferencesGeneralID, preferencesGeneralPage, "General", "Language, text size, and startup", "语言、文字大小和启动选项"},
		{preferencesEditingID, preferencesEditingPage, "Editing", "Tabs, indentation, and brackets", "制表符、缩进和括号"},
		{preferencesViewID, preferencesViewPage, "View", "Appearance and editor display", "主题和编辑器显示"},
		{preferencesSymbolsID, preferencesSymbolsPage, "Symbols", "Non-printing characters", "不可打印字符"},
		{preferencesPrintID, preferencesPrintPage, "Print", "Page layout, colors, and margins", "页面布局、颜色和页边距"},
		{preferencesPrintBandsID, preferencesPrintBandsPage, "Header/Footer", "Page headers, footers, and variables", "页眉、页脚和变量"},
		{preferencesPrintFontsID, preferencesPrintFontPage, "Print Fonts", "Header and footer typography", "页眉和页脚的字号与样式"},
		{preferencesAboutID, preferencesAboutPage, "About & Updates", "VivaPad build and software updates", "VivaPad 构建信息和软件更新"},
	}
	active := categories[0]
	navHeight := min(float32(34), max(0, (height-94)/8))
	for index, category := range categories {
		bounds := gfx.Rect{X: x + 10, Y: y + 52 + float32(index)*(navHeight+4), Width: sidebar - 20, Height: navHeight}
		selected := state.preferencesPage == category.page
		ink, weight := palette.text, gfx.WeightRegular
		if selected {
			active = category
			ink, weight = palette.accent, gfx.WeightSemibold
			canvas.Rounded(bounds, 8, palette.selectedRow)
			canvas.Rounded(gfx.Rect{X: bounds.X + 4, Y: bounds.Y + (navHeight-16)/2, Width: 3, Height: 16}, 1.5, palette.accent)
		} else if canvas.Hovered() == category.id {
			canvas.Rounded(bounds, 8, palette.toolbarHover)
		}
		if canvas.FocusVisible() == category.id {
			canvas.RoundedBorder(bounds, 8, 1, palette.accent, gfx.Color{})
		}
		label := state.preferenceText(category.title)
		state.settingsLabel(canvas, label, bounds.Inset(gfx.Insets{Left: 14, Right: 5}), ink, 13, weight)
		canvas.HitWithOptions(category.id, bounds, label, gfx.HitOptions{Role: gfx.RoleTab, Selected: selected})
	}
	for _, category := range categories {
		if canvas.Clicked(category.id) {
			state.preferencesPage, active = category.page, category
			state.resetPreferencesPending, state.printVariableOpen = false, false
			state.preferencesNotice = ""
		}
	}

	contentX := x + sidebar + 19
	contentWidth := width - sidebar - 37
	headerHeight := min(float32(76), 56+(height-352)*.18)
	footerHeight := float32(46)
	footerY := y + height - footerHeight
	state.settingsLabel(canvas, state.preferenceText(active.title), gfx.Rect{X: contentX, Y: y + headerHeight - 53, Width: contentWidth, Height: 24}, palette.text, 17, gfx.WeightSemibold)
	state.interfaceLabel(canvas, state.tr(active.detail, active.zh), gfx.Rect{X: contentX, Y: y + headerHeight - 26, Width: contentWidth, Height: 20}, palette.muted, 12)
	body := gfx.Rect{X: contentX, Y: y + headerHeight, Width: contentWidth, Height: footerY - (y + headerHeight) - 10}
	rowHeight := min(float32(44), max(0, (body.Height-12)/7))
	canvas.Clip(body)
	switch active.page {
	case preferencesEditingPage:
		state.paintEditingPreferences(canvas, body, rowHeight)
	case preferencesViewPage:
		state.paintViewPreferences(canvas, body, rowHeight)
	case preferencesPrintPage:
		state.paintPrintPreferences(canvas, body, rowHeight)
	case preferencesPrintBandsPage:
		state.paintPrintBandsPreferences(canvas, body, rowHeight)
	case preferencesPrintFontPage:
		state.paintPrintFontPreferences(canvas, body, rowHeight)
	case preferencesSymbolsPage:
		state.paintSymbolPreferences(canvas, body, rowHeight)
	case preferencesGeneralPage:
		state.paintGeneralPreferences(canvas, body, rowHeight)
	case preferencesAboutPage:
		state.paintUpdatePreferences(canvas, body)
	}
	canvas.Unclip()
	canvas.Fill(gfx.Rect{X: x + sidebar + 1, Y: footerY, Width: width - sidebar - 2, Height: 1}, palette.border)
	footer := gfx.Rect{X: contentX, Y: footerY, Width: contentWidth, Height: footerHeight}
	closeBounds := settingsControl(footer, 82, 29)
	footer.Width -= closeBounds.Width + 12
	state.interfaceLabel(canvas, state.preferencesNotice, footer, palette.muted, 11)
	if state.settingsButton(canvas, preferencesCloseID, closeBounds, state.tr("Close", "关闭"), false) {
		state.closePreferences()
	}
	canvas.PopLayer()
	canvas.PopLayer()
}

type settingsCard struct {
	bounds gfx.Rect
	line   gfx.Color
}

const settingsCardGap = 10

func (state *model) settingsCard(canvas *gfx.Canvas, bounds gfx.Rect, height float32) settingsCard {
	bounds.Height = height
	palette := state.palette()
	canvas.RoundedBorder(bounds, 10, 1, palette.border, palette.dock)
	return settingsCard{bounds: bounds, line: palette.border}
}

func (card settingsCard) row(canvas *gfx.Canvas, top, height float32) gfx.Rect {
	row := gfx.Rect{X: card.bounds.X + 12, Y: card.bounds.Y + top, Width: card.bounds.Width - 24, Height: height}
	if top > 0 {
		canvas.Fill(gfx.Rect{X: row.X, Y: row.Y, Width: row.Width, Height: 1}, card.line)
	}
	return row
}

func settingsControl(row gfx.Rect, width, height float32) gfx.Rect {
	width = min(width, row.Width)
	return gfx.Rect{X: row.X + row.Width - width, Y: row.Y + (row.Height-height)/2, Width: width, Height: height}
}

func (state *model) settingsLabel(canvas *gfx.Canvas, label string, bounds gfx.Rect, ink gfx.Color, size, weight float32) {
	size = state.interfaceSize(size)
	canvas.Label(canvas.FitLine(label, size, weight, bounds.Width), bounds, gfx.TextStyle{Color: ink, Size: size, Weight: weight})
}

func (state *model) settingsRowLabel(canvas *gfx.Canvas, label string, row, control gfx.Rect) {
	row.Width = control.X - row.X - 8
	state.settingsLabel(canvas, label, row, state.palette().text, 13, gfx.WeightMedium)
}

func (state *model) paintEditingPreferences(canvas *gfx.Canvas, body gfx.Rect, height float32) {
	card := state.settingsCard(canvas, body, height*4)
	widths := []int{2, 4, 8}
	widthLabels := []string{"2", "4", "8"}
	widthHits := []string{"Tab width: 2", "Tab width: 4", "Tab width: 8"}
	if state.chineseUI {
		widthHits = widthLabels
	}
	if index := state.settingsSegments(canvas, card.row(canvas, 0, height), state.preferenceText("Tab width"), preferencesTabScope, widthLabels, widthHits, slices.Index(widths, state.tabWidth)); index >= 0 {
		state.command("Tab Width: " + widthLabels[index])
	}
	indentLabels := []string{"Language", "Tabs", "Spaces"}
	indentHits := []string{"Indent: Language", "Indent: Tabs", "Indent: Spaces"}
	if state.chineseUI {
		indentHits = state.settingsTranslations(indentLabels)
	}
	if index := state.settingsSegments(canvas, card.row(canvas, height, height), state.preferenceText("Indent using"), preferencesIndentScope,
		state.settingsTranslations(indentLabels), indentHits, slices.Index([]string{"language", "tabs", "spaces"}, state.indentMode)); index >= 0 {
		state.command([]string{"Indent: Language Default", "Indent: Tabs", "Indent: Spaces"}[index])
	}
	state.preferenceToggle(canvas, preferencesAutoIndentID, card.row(canvas, height*2, height), "Auto Indent", state.autoIndent, "Auto Indent")
	state.preferenceToggle(canvas, preferencesPairBracketsID, card.row(canvas, height*3, height), "Pair Brackets", state.autoClose, "Pair Brackets")
}

func (state *model) paintViewPreferences(canvas *gfx.Canvas, body gfx.Rect, height float32) {
	var labels, hits [len(themeOptions)]string
	selected := -1
	for index, option := range themeOptions {
		labels[index] = state.preferenceText(option.label)
		hits[index] = state.tr(option.command(), labels[index])
		if state.themeChoice.Normalized() == option.choice {
			selected = index
		}
	}
	title := state.tr("Theme", "主题")
	extra := state.settingsSegmentExtra(canvas, body, title, labels[:])
	height = min(height, max(0, (body.Height-12-extra)/7))
	appearance := state.settingsCard(canvas, body, height+extra)
	row, title := state.settingsSegmentRow(canvas, appearance, height, extra, title)
	if index := state.settingsSegments(canvas, row, title, preferencesThemeScope, labels[:], hits[:], selected); index >= 0 {
		state.command(themeOptions[index].command())
	}
	body.Y += appearance.bounds.Height + settingsCardGap
	options := [...]struct {
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
	}
	card := state.settingsCard(canvas, body, height*float32(len(options)))
	for index, option := range options {
		state.preferenceToggle(canvas, option.id, card.row(canvas, float32(index)*height, height), option.label, option.checked, option.command)
	}
}

func (state *model) paintSymbolPreferences(canvas *gfx.Canvas, body gfx.Rect, height float32) {
	card := state.settingsCard(canvas, body, height*2)
	labels := state.settingsTranslations([]string{"Original", "Abbreviation", "Codepoint"})
	commands := []string{"Non-Printing Mode: Original", "Non-Printing Mode: Abbreviation", "Non-Printing Mode: Codepoint"}
	hits := commands
	if state.chineseUI {
		hits = labels
	}
	if index := state.settingsSegments(canvas, card.row(canvas, 0, height), state.preferenceText("Non-printing style"), preferencesNPCModeScope,
		labels, hits, slices.Index([]string{"identity", "abbreviation", "codepoint"}, state.npcMode)); index >= 0 {
		state.command(commands[index])
	}
	state.preferenceToggle(canvas, preferencesNPCIncludeID, card.row(canvas, height, height), "Include control / Unicode EOL", state.npcIncludeControls, "Include Control Characters in NPC Mode")
}

func (state *model) settingsTranslations(labels []string) []string {
	translated := make([]string, len(labels))
	for index, label := range labels {
		translated[index] = state.preferenceText(label)
	}
	return translated
}

func (state *model) settingsSegmentWidths(canvas *gfx.Canvas, labels []string) ([]float32, float32) {
	widths := make([]float32, len(labels))
	width := float32(4)
	for index, label := range labels {
		widths[index] = max(30, canvas.TextWidthWeight(label, state.interfaceSize(11), gfx.WeightSemibold)+14)
		width += widths[index]
	}
	return widths, width
}

func (state *model) settingsSegmentSpace(canvas *gfx.Canvas, rowWidth float32, title string) float32 {
	if title == "" {
		return rowWidth
	}
	return rowWidth - canvas.TextWidthWeight(title, state.interfaceSize(13), gfx.WeightMedium) - 10
}

func (state *model) settingsSegmentExtra(canvas *gfx.Canvas, body gfx.Rect, title string, labels []string) float32 {
	_, width := state.settingsSegmentWidths(canvas, labels)
	if width > state.settingsSegmentSpace(canvas, body.Width-24, title) {
		return 24
	}
	return 0
}

func (state *model) settingsSegmentRow(canvas *gfx.Canvas, card settingsCard, height, extra float32, title string) (gfx.Rect, string) {
	row := card.row(canvas, 0, height+extra)
	if extra > 0 {
		label := gfx.Rect{X: row.X, Y: row.Y + 5, Width: row.Width, Height: extra - 5}
		state.settingsLabel(canvas, title, label, state.palette().text, 13, gfx.WeightMedium)
		row.Y += extra
		row.Height -= extra
		return row, ""
	}
	return row, title
}

func (state *model) settingsSegments(canvas *gfx.Canvas, row gfx.Rect, title string, scope gfx.Scope, labels, hits []string, selected int) int {
	palette := state.palette()
	size := state.interfaceSize(11)
	widths, width := state.settingsSegmentWidths(canvas, labels)
	track := settingsControl(row, min(width, max(4, state.settingsSegmentSpace(canvas, row.Width, title))), 27)
	if title != "" {
		state.settingsRowLabel(canvas, title, row, track)
	}
	canvas.Rounded(track, 8, palette.tab)
	position := track.X + 2
	clicked := -1
	for index, label := range labels {
		itemWidth := widths[index] * (track.Width - 4) / (width - 4)
		item := gfx.Rect{X: position, Y: track.Y + 2, Width: itemWidth, Height: track.Height - 4}
		position += itemWidth
		id := scope.At(index)
		ink, weight := palette.text, gfx.WeightRegular
		if index == selected {
			ink, weight = palette.accent, gfx.WeightSemibold
			canvas.Elevate(item, 6, gfx.ShadowXS)
			canvas.Rounded(item, 6, palette.popup)
		} else if canvas.Hovered() == id {
			canvas.Rounded(item, 6, palette.toolbarHover)
		}
		if canvas.FocusVisible() == id {
			canvas.RoundedBorder(item, 6, 1, palette.accent, gfx.Color{})
		}
		canvas.LabelCenter(canvas.FitLine(label, size, weight, item.Width-6), item, gfx.TextStyle{Color: ink, Size: size, Weight: weight})
		canvas.HitWithOptions(id, item, hits[index], gfx.HitOptions{Role: gfx.RoleRadio, Checked: index == selected})
		if index != selected && canvas.Clicked(id) {
			clicked = index
		}
	}
	return clicked
}

func (state *model) preferenceToggle(canvas *gfx.Canvas, id gfx.ID, row gfx.Rect, label string, checked bool, command string) {
	label = state.preferenceText(label)
	if state.paintPreferenceToggle(canvas, id, row, label, label, checked) {
		state.command(command)
	}
}

func (state *model) paintPreferenceToggle(canvas *gfx.Canvas, id gfx.ID, row gfx.Rect, text, label string, checked bool) bool {
	palette := state.palette()
	if canvas.Hovered() == id {
		canvas.Rounded(row.Inset(gfx.Insets{Top: 3, Bottom: 3}), 6, palette.toolbarHover)
	}
	track := settingsControl(row, 34, 20)
	fill := palette.border
	if checked {
		fill = palette.accent
	}
	canvas.Rounded(track, 10, fill)
	thumb := gfx.Rect{X: track.X + 3, Y: track.Y + 3, Width: 14, Height: 14}
	if checked {
		thumb.X = track.X + track.Width - 17
	}
	canvas.Elevate(thumb, 7, gfx.ShadowXS)
	canvas.Rounded(thumb, 7, gfx.RGB24(0xffffff))
	if canvas.FocusVisible() == id {
		ring := gfx.Rect{X: track.X - 2, Y: track.Y - 2, Width: track.Width + 4, Height: track.Height + 4}
		canvas.RoundedBorder(ring, 12, 1, palette.accent, gfx.Color{})
	}
	state.settingsRowLabel(canvas, text, row, track)
	canvas.HitWithOptions(id, row, label, gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: checked})
	return canvas.Clicked(id)
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
