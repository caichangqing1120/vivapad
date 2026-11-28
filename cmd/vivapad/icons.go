package main

import (
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/figmaicons"
)

func controlIconBounds(bounds gfx.Rect) gfx.Rect {
	return centeredIconBounds(bounds, 16)
}

func centeredIconBounds(bounds gfx.Rect, size float32) gfx.Rect {
	return gfx.Rect{X: bounds.X + (bounds.Width-size)/2, Y: bounds.Y + (bounds.Height-size)/2, Width: size, Height: size}
}

var (
	toolbarThemeID    = tree.ID(page, "toolbar-quick-theme")
	toolbarSettingsID = tree.ID(page, "toolbar-quick-settings")
	moonIcon          = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(20.9, 13.35), gfx.CubicTo(17.5, 16.2, 12.2, 15.1, 10.2, 11.1),
		gfx.CubicTo(9, 8.6, 9.4, 5.7, 11, 3.5), gfx.CubicTo(5.5, 4, 2.2, 9.5, 3.8, 14.9),
		gfx.CubicTo(5.3, 20.2, 11.3, 22.7, 16.2, 20.1), gfx.CubicTo(18.9, 18.7, 20.5, 16.1, 20.9, 13.35), gfx.ClosePath(),
	}}
	sunIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(16, 12), gfx.CubicTo(16, 17.33, 8, 17.33, 8, 12), gfx.CubicTo(8, 6.67, 16, 6.67, 16, 12), gfx.ClosePath(),
		gfx.MoveTo(12, 2), gfx.LineTo(12, 4), gfx.MoveTo(12, 20), gfx.LineTo(12, 22),
		gfx.MoveTo(2, 12), gfx.LineTo(4, 12), gfx.MoveTo(20, 12), gfx.LineTo(22, 12),
		gfx.MoveTo(5, 5), gfx.LineTo(6.4, 6.4), gfx.MoveTo(17.6, 17.6), gfx.LineTo(19, 19),
		gfx.MoveTo(5, 19), gfx.LineTo(6.4, 17.6), gfx.MoveTo(17.6, 6.4), gfx.LineTo(19, 5),
	}}
	settingsIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(4, 5), gfx.LineTo(9, 5), gfx.MoveTo(15, 5), gfx.LineTo(20, 5),
		gfx.MoveTo(4, 12), gfx.LineTo(13, 12), gfx.MoveTo(19, 12), gfx.LineTo(20, 12),
		gfx.MoveTo(4, 19), gfx.LineTo(6, 19), gfx.MoveTo(12, 19), gfx.LineTo(20, 19),
		gfx.MoveTo(15, 5), gfx.CubicTo(15, 9, 9, 9, 9, 5), gfx.CubicTo(9, 1, 15, 1, 15, 5), gfx.ClosePath(),
		gfx.MoveTo(19, 12), gfx.CubicTo(19, 16, 13, 16, 13, 12), gfx.CubicTo(13, 8, 19, 8, 19, 12), gfx.ClosePath(),
		gfx.MoveTo(12, 19), gfx.CubicTo(12, 23, 6, 23, 6, 19), gfx.CubicTo(6, 15, 12, 15, 12, 19), gfx.ClosePath(),
	}}
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
	plusIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(12, 5), gfx.LineTo(12, 19),
		gfx.MoveTo(5, 12), gfx.LineTo(19, 12),
	}}
	fileIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(6, 3), gfx.LineTo(14, 3), gfx.LineTo(19, 8), gfx.LineTo(19, 21), gfx.LineTo(6, 21), gfx.ClosePath(),
		gfx.MoveTo(14, 3), gfx.LineTo(14, 8), gfx.LineTo(19, 8),
	}}
)
