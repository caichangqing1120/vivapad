package main

import "github.com/vivaui/viva/ui"

var (
	newIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(6, 3), ui.LineTo(14, 3), ui.LineTo(19, 8), ui.LineTo(19, 13),
		ui.MoveTo(6, 3), ui.LineTo(6, 21), ui.LineTo(12, 21),
		ui.MoveTo(14, 3), ui.LineTo(14, 8), ui.LineTo(19, 8),
	}}
	newAccentIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(18, 16), ui.LineTo(18, 22), ui.MoveTo(15, 19), ui.LineTo(21, 19),
	}}
	openIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(3, 19), ui.LineTo(5, 10), ui.LineTo(11, 10), ui.LineTo(13, 12), ui.LineTo(21, 12), ui.LineTo(19, 19), ui.ClosePath(),
		ui.MoveTo(4, 10), ui.LineTo(4, 5), ui.LineTo(10, 5), ui.LineTo(12, 7), ui.LineTo(19, 7), ui.LineTo(19, 10),
	}}
	saveIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(4, 3), ui.LineTo(18, 3), ui.LineTo(21, 6), ui.LineTo(21, 21), ui.LineTo(3, 21), ui.LineTo(3, 3), ui.LineTo(4, 3),
		ui.MoveTo(7, 3), ui.LineTo(7, 9), ui.LineTo(17, 9), ui.LineTo(17, 3),
		ui.MoveTo(7, 21), ui.LineTo(7, 14), ui.LineTo(17, 14), ui.LineTo(17, 21),
	}}
	searchIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(16, 10), ui.CubicTo(16, 13.3, 13.3, 16, 10, 16), ui.CubicTo(6.7, 16, 4, 13.3, 4, 10),
		ui.CubicTo(4, 6.7, 6.7, 4, 10, 4), ui.CubicTo(13.3, 4, 16, 6.7, 16, 10),
		ui.MoveTo(14.5, 14.5), ui.LineTo(21, 21),
	}}
	closeIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(6, 6), ui.LineTo(18, 18), ui.MoveTo(18, 6), ui.LineTo(6, 18),
	}}
	fullscreenRestoreIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(3, 9), ui.LineTo(9, 9), ui.LineTo(9, 3),
		ui.MoveTo(21, 9), ui.LineTo(15, 9), ui.LineTo(15, 3),
		ui.MoveTo(3, 15), ui.LineTo(9, 15), ui.LineTo(9, 21),
		ui.MoveTo(21, 15), ui.LineTo(15, 15), ui.LineTo(15, 21),
	}}
	undoIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(9, 4), ui.LineTo(4, 9), ui.LineTo(9, 14),
		ui.MoveTo(4, 9), ui.LineTo(14, 9), ui.CubicTo(19, 9, 21, 12, 21, 16), ui.CubicTo(21, 20, 18, 21, 15, 21),
	}}
	redoIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(15, 4), ui.LineTo(20, 9), ui.LineTo(15, 14),
		ui.MoveTo(20, 9), ui.LineTo(10, 9), ui.CubicTo(5, 9, 3, 12, 3, 16), ui.CubicTo(3, 20, 6, 21, 9, 21),
	}}
	cutIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(4, 5), ui.LineTo(20, 19), ui.MoveTo(20, 5), ui.LineTo(4, 19),
		ui.MoveTo(7, 5), ui.CubicTo(7, 7, 5, 8, 3, 7), ui.CubicTo(1, 6, 2, 3, 4, 3), ui.CubicTo(6, 3, 7, 4, 7, 5),
		ui.MoveTo(7, 19), ui.CubicTo(7, 21, 5, 22, 3, 21), ui.CubicTo(1, 20, 2, 17, 4, 17), ui.CubicTo(6, 17, 7, 18, 7, 19),
	}}
	copyIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(8, 7), ui.LineTo(20, 7), ui.LineTo(20, 21), ui.LineTo(8, 21), ui.ClosePath(),
		ui.MoveTo(4, 17), ui.LineTo(4, 3), ui.LineTo(16, 3),
	}}
	pasteIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(7, 5), ui.LineTo(5, 5), ui.LineTo(5, 21), ui.LineTo(19, 21), ui.LineTo(19, 5), ui.LineTo(17, 5),
		ui.MoveTo(9, 3), ui.LineTo(15, 3), ui.LineTo(16, 7), ui.LineTo(8, 7), ui.ClosePath(),
		ui.MoveTo(9, 13), ui.LineTo(15, 13),
	}}
	leftIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(15, 5), ui.LineTo(8, 12), ui.LineTo(15, 19),
	}}
	rightIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(9, 5), ui.LineTo(16, 12), ui.LineTo(9, 19),
	}}
	checkIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(4, 12), ui.LineTo(10, 18), ui.LineTo(20, 6),
	}}
	pinIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(9, 3), ui.LineTo(15, 3), ui.LineTo(16, 8), ui.LineTo(19, 11), ui.LineTo(19, 13),
		ui.LineTo(5, 13), ui.LineTo(5, 11), ui.LineTo(8, 8), ui.ClosePath(),
		ui.MoveTo(12, 13), ui.LineTo(12, 21),
	}}
	downIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(5, 9), ui.LineTo(12, 16), ui.LineTo(19, 9),
	}}
	upIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(5, 15), ui.LineTo(12, 8), ui.LineTo(19, 15),
	}}
	fileIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
		ui.MoveTo(6, 3), ui.LineTo(14, 3), ui.LineTo(19, 8), ui.LineTo(19, 21), ui.LineTo(6, 21), ui.ClosePath(),
		ui.MoveTo(14, 3), ui.LineTo(14, 8), ui.LineTo(19, 8),
	}}
)
