package main

import (
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

var multiSelectModes = [...]struct {
	label   string
	options document.SearchOptions
}{
	{"Ignore Case & Whole Word", document.SearchOptions{}},
	{"Match Case Only", document.SearchOptions{MatchCase: true}},
	{"Match Whole Word Only", document.SearchOptions{WholeWord: true}},
	{"Match Case & Whole Word", document.SearchOptions{MatchCase: true, WholeWord: true}},
}

func multiSelectModeForCommand(command string) (all bool, options document.SearchOptions, ok bool) {
	for _, mode := range multiSelectModes {
		if command == "Multi-select All / "+mode.label {
			return true, mode.options, true
		}
		if command == "Multi-select Next / "+mode.label {
			return false, mode.options, true
		}
	}
	return false, document.SearchOptions{}, false
}

func (state *model) paintMultiSelectMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	all := state.multiAllMenuOpen
	label := "Multi-select Next"
	scope := multiSelectNextScope
	if all {
		label, scope = "Multi-select All", multiSelectAllScope
	}
	width := min(float32(286), size.Width-parentRight-4)
	height := float32(len(multiSelectModes))*26 + 8
	x := parentRight
	y := max(menuHeight, size.Height-statusHeight-height-4)
	canvas.PushLayer(3)
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(scope.At(len(multiSelectModes)), panel, label, gfx.HitOptions{Role: gfx.RoleMenu})
	for index, mode := range multiSelectModes {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*26, Width: width - 8, Height: 26}
		id := scope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, mode.label, item.Inset(gfx.All(8)), palette.text, 11)
		canvas.HitWithOptions(id, item, mode.label, gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(id) {
			state.command(label + " / " + mode.label)
		}
	}
	canvas.PopLayer()
}
