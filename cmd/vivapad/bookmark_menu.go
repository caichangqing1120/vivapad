package main

import "github.com/vivaui/viva/gfx"

var bookmarkCommands = [...]string{
	"Toggle Bookmark", "Next Bookmark", "Previous Bookmark", "Clear All Bookmarks",
	"Cut Bookmarked Lines", "Copy Bookmarked Lines", "Paste to (Replace) Bookmarked Lines",
	"Remove Bookmarked Lines", "Remove Non-Bookmarked Lines", "Inverse Bookmarks",
}

func (state *model) paintBookmarkMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	width := min(float32(320), size.Width-parentRight-4)
	height := float32(len(bookmarkCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+12*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range bookmarkCommands {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := bookmarkScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(command, 11, gfx.WeightRegular, item.Width-18)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(bookmarkCommands[selected])
	}
}
