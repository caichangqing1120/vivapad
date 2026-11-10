package main

import (
	"strconv"

	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

var charsetCommands = document.LegacyEncodings()

func (state *model) paintCharsetMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(294), size.Width-8)
	available := size.Height - menuHeight - statusHeight - 8
	rowHeight := float32(22)
	scrolling := float32(len(charsetCommands))*rowHeight+8 > available
	visible := len(charsetCommands)
	controls := float32(0)
	if scrolling {
		controls = 44
		visible = max(1, int((available-controls-8)/rowHeight))
	}
	state.charsetVisible = visible
	state.charsetOffset = min(max(0, state.charsetOffset), len(charsetCommands)-visible)
	height := float32(visible)*rowHeight + controls + 8
	x := min(parentRight, size.Width-width-4)
	y := max(menuHeight, min(menuHeight+4, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	panel := ui.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(charsetPanelID, panel, "Character sets", ui.HitOptions{Role: ui.RoleMenu})
	if scrolling {
		for _, control := range []struct {
			id    ui.ID
			icon  ui.Icon
			label string
			y     float32
			step  int
		}{
			{charsetPrevID, upIcon, "Previous character sets", y + 4, -1},
			{charsetNextID, downIcon, "Next character sets", y + height - 26, 1},
		} {
			bounds := ui.Rect{X: x + 4, Y: control.y, Width: width - 8, Height: 22}
			if canvas.Hovered() == control.id {
				canvas.Fill(bounds, palette.hover)
			}
			canvas.DrawIcon(control.icon, ui.Rect{X: bounds.X + bounds.Width/2 - 8, Y: bounds.Y + 3, Width: 16, Height: 16}, palette.muted)
			canvas.HitWithOptions(control.id, bounds, control.label, ui.HitOptions{Role: ui.RoleMenuItem})
			if canvas.Clicked(control.id) {
				state.charsetOffset = min(max(0, state.charsetOffset+control.step), len(charsetCommands)-visible)
			}
		}
	}
	selected := -1
	for row := range visible {
		index := state.charsetOffset + row
		spec := charsetCommands[index]
		item := ui.Rect{X: x + 4, Y: y + 4 + controls/2 + float32(row)*rowHeight, Width: width - 8, Height: rowHeight}
		id := charsetScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		checked := state.menuOpen == 4 && state.space.Current().Encoding() == spec.Encoding
		if checked {
			canvas.DrawIcon(checkIcon, ui.Rect{X: item.X + 5, Y: item.Y + 4, Width: 14, Height: 14}, palette.blue)
		}
		label := canvas.FitLine(spec.Group+" / "+spec.Label, 11, ui.WeightRegular, item.Width-38)
		smallLabel(canvas, label, ui.Rect{X: item.X + 26, Y: item.Y, Width: item.Width - 35, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, spec.Label, ui.HitOptions{Role: ui.RoleMenuItem, Checked: checked, Hint: spec.Group + " code page " + strconv.Itoa(spec.CodePage)})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected < 0 {
		return
	}
	openFile := state.menuOpen == 0
	state.menuOpen, state.charsetMenuOpen = -1, false
	if openFile {
		state.cancelMacroForCommand()
		state.openFilesWithEncoding(charsetCommands[selected].Encoding)
	} else {
		state.command(charsetCommands[selected].Label)
	}
}
