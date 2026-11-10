package main

import "github.com/vivaui/viva/ui"

func (state *model) paintMarkJumpMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	mode := "down"
	if state.markJumpPrevious {
		mode = "up"
	}
	state.paintStylePopup(canvas, size, parentRight, mode)
}
