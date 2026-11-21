package main

import "github.com/vivaui/viva/gfx"

func (state *model) paintMarkJumpMenu(canvas *gfx.Canvas, size gfx.Size, parentRight, parentY float32) {
	mode := "down"
	if state.markJumpPrevious {
		mode = "up"
	}
	state.paintStylePopup(canvas, size, parentRight, parentY, mode)
}
