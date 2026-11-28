package main

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/syntax"
)

var tabColorCommands = [...]string{"Apply Color 1", "Apply Color 2", "Apply Color 3", "Apply Color 4", "Apply Color 5", "Remove Color"}

// Classic light colors from Notepad++ v8.9.8.1, in tabColourId order.
// Store the Windows HLS results so the same defaults paint on every platform.
var tabColorPastels = [...]gfx.Color{
	gfx.RGB24(0xf3f0cb), gfx.RGB24(0xd6f3cb), gfx.RGB24(0xcbdbf3),
	gfx.RGB24(0xf3dacb), gfx.RGB24(0xeecbf3),
}

var tabColorAccents = [...]gfx.Color{
	gfx.RGB24(0xe4d945), gfx.RGB24(0x6de445), gfx.RGB24(0x4585e4),
	gfx.RGB24(0xe48145), gfx.RGB24(0xd045e4),
}

func tabColorCommand(command string) uint8 {
	for index, name := range tabColorCommands {
		if name == command {
			if index == 5 {
				return 0
			}
			return uint8(index + 1)
		}
	}
	return 0
}

func (state *model) setTabColor(command string) {
	doc := state.paneDocument(state.activePane)
	if state.documentIndex(doc) < 0 {
		return
	}
	color := tabColorCommand(command)
	if color == 0 {
		delete(state.tabColors, doc)
	} else {
		state.tabColors[doc] = color
	}
	state.invalidator.Invalidate()
}

func (state *model) restoreTabColors(colors map[string]uint8) {
	for _, doc := range state.space.Documents {
		if color := colors[doc.Path]; color >= 1 && color <= 5 {
			state.tabColors[doc] = color
		}
	}
}

func (state *model) tabColorPaint(doc *document.Document, selected bool) (background, accent gfx.Color) {
	palette := state.palette()
	background, accent = palette.tab, palette.accent
	if selected {
		background = palette.activeTab
	}
	if color := state.tabColors[doc]; color >= 1 && color <= 5 {
		accent = tabColorAccents[color-1]
		if !selected {
			background = state.tabColorSwatch(color)
		}
	}
	return background, accent
}

func (state *model) paintTabFace(canvas *gfx.Canvas, doc *document.Document, bounds gfx.Rect, tab, pin, close gfx.ID, label string, selected bool) {
	palette := state.palette()
	background, accent := state.tabColorPaint(doc, selected)
	if selected {
		rounded := bounds
		rounded.Height += 6
		canvas.Clip(bounds)
		canvas.RoundedBorder(rounded, 6, 1, palette.chromeLine, palette.editorBackground)
		canvas.Clip(gfx.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: 2})
		canvas.Rounded(rounded, 6, accent)
		canvas.Unclip()
		canvas.Unclip()
	} else if hover := canvas.Hovered(); hover == tab || hover == pin || hover == close {
		canvas.Rounded(bounds.Inset(gfx.Insets{Bottom: 2}), 6, palette.toolbarHover)
	} else if state.tabColors[doc] != 0 {
		canvas.Rounded(bounds.Inset(gfx.Insets{Bottom: 2}), 6, background)
	}
	textBounds := gfx.Rect{X: bounds.X + 10, Y: bounds.Y, Width: bounds.Width - 56, Height: bounds.Height - 1}
	if doc.Dirty() {
		canvas.Rounded(gfx.Rect{X: textBounds.X, Y: textBounds.Y + (textBounds.Height-6)/2, Width: 6, Height: 6}, 3, palette.accent)
		textBounds.X += 12
		textBounds.Width -= 12
	}
	state.interfaceLabel(canvas, label, textBounds, palette.text, 11)
}

func (state *model) tabColorSwatch(color uint8) gfx.Color {
	if state.renderTheme == syntax.ClassicLight {
		return tabColorPastels[color-1]
	}
	tint := tabColorAccents[color-1]
	tint.Alpha = 0.22
	return tint.Over(state.palette().tab)
}

func (state *model) tabColorHint(doc *document.Document) string {
	if color := state.tabColors[doc]; color >= 1 && color <= 5 {
		return tabColorCommands[color-1]
	}
	return ""
}

func (state *model) tabColorMenuRow(canvas *gfx.Canvas, item gfx.Rect, command string, checked bool) {
	palette := state.palette()
	if command == "Remove Color" {
		canvas.DrawIcon(closeIcon, gfx.Rect{X: item.X + 8, Y: item.Y + 5, Width: 12, Height: 12}, palette.muted)
		return
	}
	color := tabColorCommand(command)
	if color > 0 {
		canvas.Border(gfx.Rect{X: item.X + 8, Y: item.Y + 4, Width: 14, Height: 14}, 1, palette.border, state.tabColorSwatch(color))
		if checked {
			canvas.Fill(gfx.Rect{X: item.X + 9, Y: item.Y + 17, Width: 12, Height: 2}, tabColorAccents[color-1])
		}
	}
}

func isTabColorCommand(command string) bool {
	return strings.HasPrefix(command, "Apply Color ") && tabColorCommand(command) != 0 || command == "Remove Color"
}
