package main

import (
	"github.com/vivaui/viva/ui"
	"vivapad/internal/figmaicons"
)

func controlIconBounds(bounds ui.Rect) ui.Rect {
	return ui.Rect{X: bounds.X + (bounds.Width-16)/2, Y: bounds.Y + (bounds.Height-16)/2, Width: 16, Height: 16}
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
	newAccentIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(18, 16), ui.LineTo(18, 22), ui.MoveTo(15, 19), ui.LineTo(21, 19),
	}}
	fullscreenRestoreIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(3, 9), ui.LineTo(9, 9), ui.LineTo(9, 3),
		ui.MoveTo(21, 9), ui.LineTo(15, 9), ui.LineTo(15, 3),
		ui.MoveTo(3, 15), ui.LineTo(9, 15), ui.LineTo(9, 21),
		ui.MoveTo(21, 15), ui.LineTo(15, 15), ui.LineTo(15, 21),
	}}
	checkIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(4, 12), ui.LineTo(10, 18), ui.LineTo(20, 6),
	}}
	fileIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(6, 3), ui.LineTo(14, 3), ui.LineTo(19, 8), ui.LineTo(19, 21), ui.LineTo(6, 21), ui.ClosePath(),
		ui.MoveTo(14, 3), ui.LineTo(14, 8), ui.LineTo(19, 8),
	}}
)
