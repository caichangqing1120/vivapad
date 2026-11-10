package main

import (
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

var lineCommands = [...]string{
	"Sort Lines Ascending", "Sort Lines Descending", "Remove Duplicate Lines",
	"Sort Ascending (Ignore Case)", "Sort Descending (Ignore Case)",
	"Remove Empty Lines", "Remove Blank Lines", "Remove Consecutive Duplicates",
	"Join Lines", "Reverse Line Order", "Insert Blank Line Above Current", "Insert Blank Line Below Current",
	"Split Lines",
	"Sort As Integers Ascending", "Sort As Integers Descending",
	"Sort As Decimals (Comma) Ascending", "Sort As Decimals (Comma) Descending",
	"Sort As Decimals (Dot) Ascending", "Sort As Decimals (Dot) Descending",
	"Sort By Length Ascending", "Sort By Length Descending",
	"Sort In Locale Order Ascending", "Sort In Locale Order Descending", "Randomize Line Order",
}

func numericSortForCommand(command string) (document.SortMode, bool, bool) {
	switch command {
	case "Sort As Integers Ascending":
		return document.SortIntegerAware, true, true
	case "Sort As Integers Descending":
		return document.SortIntegerAware, false, true
	case "Sort As Decimals (Comma) Ascending":
		return document.SortDecimalComma, true, true
	case "Sort As Decimals (Comma) Descending":
		return document.SortDecimalComma, false, true
	case "Sort As Decimals (Dot) Ascending":
		return document.SortDecimalDot, true, true
	case "Sort As Decimals (Dot) Descending":
		return document.SortDecimalDot, false, true
	case "Sort By Length Ascending":
		return document.SortLength, true, true
	case "Sort By Length Descending":
		return document.SortLength, false, true
	}
	return 0, false, false
}

func (state *model) paintLineMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(310), size.Width-parentRight-4)
	available := size.Height - menuHeight - statusHeight - 8
	rowHeight := float32(24)
	scrolling := float32(len(lineCommands))*rowHeight+8 > available
	visible := len(lineCommands)
	if scrolling {
		rowHeight = 21
		visible = max(1, int((available-52)/rowHeight))
	}
	state.lineMenuVisible = visible
	state.lineMenuOffset = min(max(0, state.lineMenuOffset), len(lineCommands)-visible)
	controls := float32(0)
	if scrolling {
		controls = 44
	}
	height := float32(visible)*rowHeight + controls + 8
	x := parentRight
	y := min(menuHeight+4+10*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	panel := ui.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	canvas.HitWithOptions(linePanelID, panel, "Line Operations", ui.HitOptions{Role: ui.RoleMenu})
	if scrolling {
		previous := ui.Rect{X: x + 4, Y: y + 4, Width: width - 8, Height: 22}
		next := ui.Rect{X: x + 4, Y: y + height - 26, Width: width - 8, Height: 22}
		if canvas.Hovered() == linePrevID {
			canvas.Fill(previous, palette.hover)
		}
		if canvas.Hovered() == lineNextID {
			canvas.Fill(next, palette.hover)
		}
		canvas.DrawIcon(upIcon, ui.Rect{X: previous.X + previous.Width/2 - 8, Y: previous.Y + 3, Width: 16, Height: 16}, palette.muted)
		canvas.DrawIcon(downIcon, ui.Rect{X: next.X + next.Width/2 - 8, Y: next.Y + 3, Width: 16, Height: 16}, palette.muted)
		if hitButton(canvas, linePrevID, previous, "Previous line operations") {
			state.lineMenuOffset = max(0, state.lineMenuOffset-1)
		}
		if hitButton(canvas, lineNextID, next, "Next line operations") {
			state.lineMenuOffset = min(len(lineCommands)-visible, state.lineMenuOffset+1)
		}
	}
	selected := -1
	for row := range visible {
		index := state.lineMenuOffset + row
		command := lineCommands[index]
		item := ui.Rect{X: x + 4, Y: y + 4 + controls/2 + float32(row)*rowHeight, Width: width - 8, Height: rowHeight}
		id := lineScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(command, 11, ui.WeightRegular, item.Width-18)
		smallLabel(canvas, label, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, ui.HitOptions{Role: ui.RoleMenuItem, Hint: command})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(lineCommands[selected])
	}
}
