package main

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

var tabColorCommands = [...]string{"Apply Color 1", "Apply Color 2", "Apply Color 3", "Apply Color 4", "Apply Color 5", "Remove Color"}

var tabColorPastels = [...]gfx.Color{
	gfx.RGB(249, 224, 205), gfx.RGB(245, 235, 203), gfx.RGB(219, 239, 218),
	gfx.RGB(221, 232, 251), gfx.RGB(238, 222, 246),
}

var tabColorAccents = [...]gfx.Color{
	gfx.RGB(207, 107, 53), gfx.RGB(173, 139, 42), gfx.RGB(66, 156, 81),
	gfx.RGB(75, 120, 203), gfx.RGB(151, 88, 172),
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
	background, accent = palette.tab, palette.blue
	if selected {
		background = palette.white
	}
	if color := state.tabColors[doc]; color >= 1 && color <= 5 {
		accent = tabColorAccents[color-1]
		if !selected {
			background = tabColorPastels[color-1]
		}
	}
	return background, accent
}

func (state *model) tabColorHint(doc *document.Document) string {
	if color := state.tabColors[doc]; color >= 1 && color <= 5 {
		return tabColorCommands[color-1]
	}
	return ""
}

func (state *model) tabColorMenuRow(canvas *gfx.Canvas, item gfx.Rect, command string, checked bool) {
	if command == "Remove Color" {
		canvas.DrawIcon(closeIcon, gfx.Rect{X: item.X + 8, Y: item.Y + 5, Width: 12, Height: 12}, palette.muted)
		return
	}
	color := tabColorCommand(command)
	if color > 0 {
		canvas.Border(gfx.Rect{X: item.X + 8, Y: item.Y + 4, Width: 14, Height: 14}, 1, palette.border, tabColorPastels[color-1])
		if checked {
			canvas.Fill(gfx.Rect{X: item.X + 9, Y: item.Y + 17, Width: 12, Height: 2}, tabColorAccents[color-1])
		}
	}
}

func isTabColorCommand(command string) bool {
	return strings.HasPrefix(command, "Apply Color ") && tabColorCommand(command) != 0 || command == "Remove Color"
}
