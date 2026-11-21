package main

import (
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/figmaicons"
)

func controlIconBounds(bounds gfx.Rect) gfx.Rect {
	return gfx.Rect{X: bounds.X + (bounds.Width-16)/2, Y: bounds.Y + (bounds.Height-16)/2, Width: 16, Height: 16}
}

var (
	newIcon       = figmaicons.New
	openIcon      = figmaicons.Open
	saveIcon      = figmaicons.Save
	searchIcon    = figmaicons.Search
	closeIcon     = figmaicons.Close
	undoIcon      = figmaicons.Undo
	redoIcon      = figmaicons.Redo
	cutIcon       = figmaicons.Cut
	copyIcon      = figmaicons.Copy
	pasteIcon     = figmaicons.Paste
	leftIcon      = figmaicons.Left
	rightIcon     = figmaicons.Right
	pinIcon       = figmaicons.Pin
	downIcon      = figmaicons.Down
	upIcon        = figmaicons.Up
	newAccentIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(18, 16), gfx.LineTo(18, 22), gfx.MoveTo(15, 19), gfx.LineTo(21, 19),
	}}
	fullscreenRestoreIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(3, 9), gfx.LineTo(9, 9), gfx.LineTo(9, 3),
		gfx.MoveTo(21, 9), gfx.LineTo(15, 9), gfx.LineTo(15, 3),
		gfx.MoveTo(3, 15), gfx.LineTo(9, 15), gfx.LineTo(9, 21),
		gfx.MoveTo(21, 15), gfx.LineTo(15, 15), gfx.LineTo(15, 21),
	}}
	checkIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(4, 12), gfx.LineTo(10, 18), gfx.LineTo(20, 6),
	}}
	fileIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(6, 3), gfx.LineTo(14, 3), gfx.LineTo(19, 8), gfx.LineTo(19, 21), gfx.LineTo(6, 21), gfx.ClosePath(),
		gfx.MoveTo(14, 3), gfx.LineTo(14, 8), gfx.LineTo(19, 8),
	}}
)
