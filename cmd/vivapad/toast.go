package main

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

const toastDuration = 2400 * time.Millisecond

type toastState struct {
	message string
	until   time.Time
}

func (state *model) showToast(message string) {
	state.toast = toastState{message: message, until: time.Now().Add(toastDuration)}
	state.invalidator.Invalidate()
}

func (state *model) paintToast(canvas *gfx.Canvas, size gfx.Size, status float32) {
	if state.toast.message == "" {
		return
	}
	remaining := time.Until(state.toast.until)
	if remaining <= 0 {
		state.toast = toastState{}
		return
	}
	canvas.RequestFrameIn(remaining)
	if size.Width < 100 || size.Height < status+60 {
		return
	}
	colors := state.palette()
	fontSize := state.interfaceSize(12)
	width := min(float32(400), size.Width-28, canvas.TextWidth(state.toast.message, fontSize)+44)
	bounds := gfx.Rect{X: size.Width - width - 14, Y: size.Height - status - 46, Width: width, Height: 34}
	canvas.PushLayer(8)
	canvas.Rounded(gfx.Rect{X: bounds.X, Y: bounds.Y + 2, Width: bounds.Width, Height: bounds.Height + 2}, 10, gfx.RGBA(0, 0, 0, .13))
	canvas.RoundedBorder(bounds, 10, 1, colors.border, colors.popup)
	canvas.DrawIcon(checkIcon, gfx.Rect{X: bounds.X + 10, Y: bounds.Y + 9, Width: 16, Height: 16}, colors.pin)
	state.interfaceLabel(canvas, state.toast.message, gfx.Rect{X: bounds.X + 32, Y: bounds.Y, Width: width - 41, Height: 34}, colors.text, 12)
	canvas.PopLayer()
}
