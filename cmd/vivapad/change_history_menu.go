package main

import (
	"github.com/vivaui/viva/gfx"
)

// H06: the Search > Change History submenu uses the original program's wording
// ("Go to ...") while the commands keep their established names.

type changeHistoryItem struct {
	label   string
	command string
}

var changeHistoryItems = [...]changeHistoryItem{
	{"Go to Next Change", "Next Change"},
	{"Go to Previous Change", "Previous Change"},
	{"Clear Change History", "Clear Change History"},
}

func (state *model) paintChangeHistoryMenu(canvas *gfx.Canvas, size gfx.Size, parentRight, parentY float32) {
	width := min(float32(300), size.Width-parentRight-4)
	height := float32(len(changeHistoryItems))*27 + 8
	x := parentRight
	y := min(parentY, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(changeHistoryScope.At(len(changeHistoryItems)), panel, "Change History", gfx.HitOptions{Role: gfx.RoleMenu})
	// An unavailable history keeps the entries visible but inert, and the reason
	// replaces "no changes" rather than pretending the document is clean.
	unavailable := ""
	if doc := state.space.Current(); doc != nil {
		if reason, bad := changeHistoryUnavailable(doc); bad {
			unavailable = reason
		}
	}
	hot := canvas.Focused()
	for index, item := range changeHistoryItems {
		if canvas.Hovered() == changeHistoryScope.At(index) && (unavailable == "" || item.command == "Clear Change History") {
			hot = canvas.Hovered()
		}
		if canvas.FocusVisible() == changeHistoryScope.At(index) {
			hot = canvas.FocusVisible()
			break
		}
	}
	selected := -1
	for index, item := range changeHistoryItems {
		rect := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := changeHistoryScope.At(index)
		enabled := unavailable == "" || item.command == "Clear Change History"
		if hot == id && enabled {
			canvas.Fill(rect, palette.hover)
		}
		color := palette.text
		if !enabled {
			color = palette.muted
		}
		label := canvas.FitLine(item.label, 11, gfx.WeightRegular, rect.Width-18)
		smallLabel(canvas, label, gfx.Rect{X: rect.X + 9, Y: rect.Y, Width: rect.Width - 18, Height: rect.Height}, color, 11)
		canvas.HitWithOptions(id, rect, item.label, gfx.HitOptions{Role: gfx.RoleMenuItem, Disabled: !enabled})
		if enabled && canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if unavailable != "" {
		// The submenu is the user's current action: its reason wins over a bookmark
		// warning that the repaint path may have written meanwhile.
		state.notice = unavailable
	}
	if selected >= 0 {
		state.command(changeHistoryItems[selected].command)
	}
}
