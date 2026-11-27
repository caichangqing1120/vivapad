package main

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/preferences"
	"vivapad/internal/syntax"
)

const styleConfiguratorCommand = "Style Configurator..."

var (
	styleSwatches   = tree.Sub(page, tree.Name("style-swatches"))
	styleRows       = tree.Sub(page, tree.Name("style-color-rows"))
	styleSizeDownID = tree.ID(page, "style-font-decrease")
	styleSizeUpID   = tree.ID(page, "style-font-increase")
	styleResetID    = tree.ID(page, "style-reset")
	styleApplyID    = tree.ID(page, "style-apply")
)

var styleLabels = [...]string{"Background", "Foreground", "Keyword", "String", "Comment", "Number", "Function"}
var stylePresets = [...]uint32{0xffffff, 0x1e1f22, 0xe5e7eb, 0x315ca7, 0x44aa88, 0xb45339, 0x9a71c8, 0xf6c568}

type editorStyleCache struct {
	base    *themePalette
	colors  preferences.StyleColors
	palette themePalette
	tokens  map[gfx.Color]gfx.Color
}

func (state *model) concreteStyleTheme() preferences.Theme {
	switch state.renderTheme {
	case syntax.ModernDark:
		return preferences.ModernDark
	case syntax.Monokai:
		return preferences.Monokai
	default:
		return preferences.ClassicLight
	}
}

func (state *model) openStyleConfigurator() {
	state.beginThemeFrame()
	theme := state.concreteStyleTheme()
	colors := state.styles.For(theme)
	size := state.paneFontSize(state.activePane)
	dialog := state.openM5Dialog(styleConfiguratorCommand, 0, colors[:]...)
	dialog.theme, dialog.size = theme, size
}

func (dialog *m5Dialog) styleColors() preferences.StyleColors {
	var colors preferences.StyleColors
	for index, field := range dialog.fields {
		colors[index] = strings.ToUpper(field.String())
	}
	return colors
}

func (state *model) styledPalette(base *themePalette) *themePalette {
	theme := state.concreteStyleTheme()
	colors := state.styles.For(theme)
	if dialog := state.m5; dialog != nil && dialog.kind == styleConfiguratorCommand && dialog.theme == theme {
		for index, color := range dialog.styleColors() {
			if _, valid := preferences.ParseStyleColor(color); valid || color == "" {
				colors[index] = color
			}
		}
	}
	if colors == (preferences.StyleColors{}) {
		state.editorStyle = nil
		return base
	}
	if cached := state.editorStyle; cached != nil && cached.base == base && cached.colors == colors {
		return &cached.palette
	}
	style := &editorStyleCache{base: base, colors: colors, palette: *base, tokens: make(map[gfx.Color]gfx.Color)}
	if color, ok := preferences.ParseStyleColor(colors[0]); ok {
		style.palette.editorBackground = gfx.RGB24(color)
	}
	if color, ok := preferences.ParseStyleColor(colors[1]); ok {
		style.palette.editorForeground = gfx.RGB24(color)
	}
	for index, token := range syntax.StyleTokenColors() {
		if color, ok := preferences.ParseStyleColor(colors[index+2]); ok {
			style.tokens[token] = gfx.RGB24(color)
		}
	}
	state.editorStyle = style
	return &style.palette
}

func (state *model) paintStyleConfigurator(canvas *gfx.Canvas, bounds gfx.Rect, dialog *m5Dialog) {
	palette := state.palette()
	label := string(dialog.theme)
	for _, option := range themeOptions {
		if option.choice == dialog.theme {
			label = option.label
		}
	}
	left, top, width := bounds.X, bounds.Y, bounds.Width
	smallLabel(canvas, "Style Configurator  /  "+label, gfx.Rect{X: left, Y: top, Width: width, Height: 24}, palette.text, 15)
	smallLabel(canvas, fmt.Sprintf("Editor font size: %d", dialog.size), gfx.Rect{X: left, Y: top + 31, Width: 144, Height: 25}, palette.text, 12)
	if palette.m5Button(canvas, styleSizeDownID, gfx.Rect{X: left + 144, Y: top + 31, Width: 28, Height: 25}, "-", dialog.size <= 9) {
		dialog.size--
		state.invalidator.Invalidate()
	}
	if palette.m5Button(canvas, styleSizeUpID, gfx.Rect{X: left + 176, Y: top + 31, Width: 28, Height: 25}, "+", dialog.size >= 24) {
		dialog.size++
		state.invalidator.Invalidate()
	}
	state.paintStyleSample(canvas, gfx.Rect{X: left + 218, Y: top + 28, Width: width - 218, Height: 34}, dialog.size)
	columnWidth := (width - 14) / 2
	colors := dialog.styleColors()
	for index, label := range styleLabels {
		column, row := index/4, index%4
		x, y := left+float32(column)*(columnWidth+14), top+69+float32(row)*32
		button := gfx.Rect{X: x, Y: y, Width: columnWidth - 100, Height: 27}
		if dialog.styleRow == index {
			canvas.Fill(button, palette.selectedRow)
		}
		smallLabel(canvas, label, button.Inset(gfx.Insets{Left: 6}), palette.text, 12)
		if hitButton(canvas, styleRows.At(index), button, label) || canvas.Clicked(m5Fields.At(index)) {
			dialog.styleRow = index
		}
		_, valid := preferences.ParseStyleColor(colors[index])
		state.paintM5Field(canvas, gfx.Rect{X: x + columnWidth - 96, Y: y, Width: 96, Height: 27}, index, label+" color", "Default", colors[index] != "" && !valid)
	}
	smallLabel(canvas, styleLabels[dialog.styleRow], gfx.Rect{X: left, Y: top + 204, Width: 96, Height: 24}, palette.muted, 11)
	for index, color := range stylePresets {
		id := styleSwatches.At(index)
		value := fmt.Sprintf("#%06X", color)
		cell := gfx.Rect{X: left + 98 + float32(index)*31, Y: top + 204, Width: 26, Height: 24}
		border := palette.border
		if canvas.FocusVisible() == id || colors[dialog.styleRow] == value {
			border = palette.accent
		}
		canvas.Border(cell, 2, border, gfx.RGB24(color))
		canvas.HitWithOptions(id, cell, value, gfx.HitOptions{Role: gfx.RoleButton, Hint: styleLabels[dialog.styleRow], Selected: colors[dialog.styleRow] == value})
		if canvas.Clicked(id) {
			dialog.fields[dialog.styleRow] = text.NewEditor(value)
			state.invalidator.Invalidate()
		}
	}
	validation := colors.Validate()
	message := dialog.message
	if validation != nil {
		message = "Use #RRGGBB or leave a field empty for the default."
	}
	smallLabel(canvas, canvas.FitLine(message, 11, gfx.WeightRegular, width), gfx.Rect{X: left, Y: top + 236, Width: width, Height: 24}, palette.diff[0], 11)
	bottom := top + bounds.Height - 29
	if palette.m5Button(canvas, styleResetID, gfx.Rect{X: left, Y: bottom, Width: 118, Height: 29}, "Restore Defaults", false) {
		for index := range dialog.fields {
			dialog.fields[index] = text.NewEditor("")
		}
		dialog.size, dialog.message = preferences.Default().FontSize, ""
		state.invalidator.Invalidate()
	}
	if palette.m5Button(canvas, styleApplyID, gfx.Rect{X: left + width - 156, Y: bottom, Width: 74, Height: 29}, "Save", validation != nil) {
		previous, main, sub := state.styles, state.fontSize, state.subFontSize
		*state.styles.ForTheme(dialog.theme) = colors
		state.fontSize, state.subFontSize = dialog.size, dialog.size
		if err := state.savePreferences(); err != nil {
			state.styles, state.fontSize, state.subFontSize = previous, main, sub
			dialog.message = state.notice
		} else {
			state.notice = "Editor style saved"
			state.closeM5Dialog()
		}
	}
}

func (state *model) paintStyleSample(canvas *gfx.Canvas, bounds gfx.Rect, fontSize int) {
	palette := state.palette()
	canvas.Border(bounds, 1, palette.border, palette.editorBackground)
	canvas.Clip(bounds.Inset(gfx.All(2)))
	defer canvas.Unclip()
	colors := syntax.StyleTokenColors()
	segments := [...]struct {
		value string
		token int
	}{{"let ", 0}, {"f", 4}, {"(", -1}, {`"x"`, 1}, {",", -1}, {"1", 3}, {") ", -1}, {"// c", 2}}
	x := bounds.X + 7
	for _, segment := range segments {
		color := palette.editorForeground
		if segment.token >= 0 {
			color = state.runStyleColor(colors[segment.token])
		}
		style := gfx.TextStyle{Size: float32(fontSize), Color: color, Mono: true}
		width := canvas.TextWidthStyle(segment.value, style)
		canvas.Label(segment.value, gfx.Rect{X: x, Y: bounds.Y, Width: width + 1, Height: bounds.Height}, style)
		x += width
	}
}

func (state *model) runStyleColor(base gfx.Color) gfx.Color {
	if style := state.editorStyle; style != nil {
		if color, ok := style.tokens[base]; ok {
			return color
		}
	}
	return state.renderTheme.TokenColor(base)
}
