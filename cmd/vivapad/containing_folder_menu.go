package main

import "github.com/vivaui/viva/ui"

var containingFolderCommands = [...]string{"Explorer", "cmd", "PowerShell", "Folder as Workspace"}

func (state *model) paintContainingFolderMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(220), size.Width-parentRight-4)
	height := float32(len(containingFolderCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+18*fileMenuRowHeight, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range containingFolderCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := containingFolderScope.At(index)
		disabled := index == 1 || index == 2
		if !disabled && canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		color := palette.text
		if disabled {
			color = palette.muted
		}
		smallLabel(canvas, command, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, color, 11)
		canvas.HitWithOptions(id, item, command, ui.HitOptions{Role: ui.RoleMenuItem, Disabled: disabled})
		if !disabled && canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected == 0 {
		state.command("Explorer")
	} else if selected == 3 {
		state.command("Folder of Current Document as Workspace")
	}
}
