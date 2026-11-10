package main

import (
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

var blankCommands = [...]string{
	"Trim Trailing Space", "Trim Leading Space", "Trim Leading and Trailing Space",
	"EOL to Space", "Trim both and EOL to Space", "TAB to Space",
	"Space to TAB (All)", "Space to TAB (Leading)",
}

func blankOperationForCommand(command string) (document.BlankOperation, bool) {
	for index, label := range blankCommands {
		if command == label {
			return document.BlankOperation(index + 1), true
		}
	}
	return 0, false
}

func (state *model) applyBlankOperation(doc *document.Document, operation document.BlankOperation) bool {
	if selection := state.rectangles[state.activePane]; selection.active && selection.doc == doc && selection.value == doc.Editor.String() {
		return false
	}
	changed := false
	state.editWithBookmarkAnchor(doc, func() { changed = doc.ApplyBlankOperation(operation) })
	return changed
}

func (state *model) paintBlankMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(315), size.Width-parentRight-4)
	height := float32(len(blankCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+12*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range blankCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := blankScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(command, 11, ui.WeightRegular, item.Width-18)
		smallLabel(canvas, label, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, ui.HitOptions{Role: ui.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(blankCommands[selected])
	}
}
