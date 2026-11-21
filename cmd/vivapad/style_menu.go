package main

import "github.com/vivaui/viva/gfx"

var styleOrdinals = [...]string{"1st", "2nd", "3rd", "4th", "5th"}

func (state *model) paintStylePopup(canvas *gfx.Canvas, size gfx.Size, parentRight, parentY float32, mode string) {
	labels := make([]string, 0, 7)
	for _, ordinal := range styleOrdinals {
		label := ordinal + " Style"
		switch mode {
		case "all", "one":
			label = "Using " + label
		case "clear":
			label = "Clear " + label
		}
		labels = append(labels, label)
	}
	scope := styleMenuScope
	switch mode {
	case "clear":
		labels = append(labels, "Clear all Styles")
	case "copy":
		labels = append(labels, "All Styles", "Find Mark Style")
	case "up", "down":
		scope = markJumpScope
		labels = append(labels, "Find Mark Style")
	}
	width := min(float32(228), size.Width-parentRight-4)
	height := float32(len(labels))*27 + 8
	x := parentRight
	y := min(parentY, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, label := range labels {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := scope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		if index < len(styleColors) {
			canvas.Border(gfx.Rect{X: item.X + 7, Y: item.Y + 7, Width: 13, Height: 13}, 1, palette.border, styleColors[index])
		}
		text := canvas.FitLine(label, 11, gfx.WeightRegular, item.Width-38)
		smallLabel(canvas, text, gfx.Rect{X: item.X + 29, Y: item.Y, Width: item.Width - 38, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, label, gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected < 0 {
		return
	}
	state.menuOpen, state.styleMenuMode, state.markJumpMenuOpen = -1, "", false
	switch mode {
	case "all", "one":
		state.styleToken(selected, mode == "all")
	case "clear":
		if selected == 5 {
			selected = -1
		}
		state.clearStyles(selected)
	case "copy":
		switch selected {
		case 5:
			state.copyStyles(-1)
		case 6:
			state.copyMarkedText()
		default:
			state.copyStyles(selected)
		}
	case "up", "down":
		if selected == 5 {
			state.navigateMarks(mode == "up")
		} else {
			state.navigateStyle(selected, mode == "up")
		}
	}
	state.focusViewRequested, state.focusViewArmed = true, false
	state.invalidator.Invalidate()
}
