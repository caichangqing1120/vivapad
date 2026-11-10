package main

import (
	"slices"

	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

var viewTabCommands = [...]string{
	"1st Tab", "2nd Tab", "3rd Tab", "4th Tab", "5th Tab", "6th Tab", "7th Tab", "8th Tab", "9th Tab",
	"First Tab", "Last Tab", "Next Tab", "Previous Tab",
	"Move to Start", "Move to End", "Move Tab Forward", "Move Tab Backward",
	"Apply Color 1", "Apply Color 2", "Apply Color 3", "Apply Color 4", "Apply Color 5", "Remove Color",
}

func (state *model) activeViewTabs() []*document.Document {
	if state.splitEnabled {
		return state.paneTabs(state.activePane)
	}
	return state.space.Documents
}

func (state *model) selectViewTab(position int) {
	tabs := state.activeViewTabs()
	if len(tabs) == 0 {
		return
	}
	position = min(max(position, 0), len(tabs)-1)
	doc := tabs[position]
	if index := state.documentIndex(doc); index >= 0 {
		state.space.Select(index)
		state.paneDocs[state.activePane] = doc
	}
}

func (state *model) moveViewTab(command string) {
	tabs := state.activeViewTabs()
	if len(tabs) < 2 {
		return
	}
	selected := state.space.Current()
	if state.splitEnabled {
		selected = state.paneDocument(state.activePane)
	}
	position := slices.Index(tabs, selected)
	if position < 0 {
		return
	}
	if state.splitEnabled && state.paneOwner[selected] != state.activePane {
		state.notice = "Shared clone order follows its owning view"
		return
	}
	firstUnpinned := len(tabs)
	for index, doc := range tabs {
		if !state.isPinned(doc) {
			firstUnpinned = index
			break
		}
	}
	start, end := firstUnpinned, len(tabs)-1
	if state.isPinned(selected) {
		start, end = 0, firstUnpinned-1
	}
	target := position
	switch command {
	case "Move to Start":
		target = start
	case "Move to End":
		target = end
	case "Move Tab Forward":
		target = min(end, position+1)
	case "Move Tab Backward":
		target = max(start, position-1)
	}
	if target == position {
		return
	}
	peer := tabs[target]
	from := state.documentIndex(selected)
	if from < 0 {
		return
	}
	state.space.Documents = append(state.space.Documents[:from], state.space.Documents[from+1:]...)
	insertAt := state.documentIndex(peer)
	if target > position {
		insertAt++
	}
	state.space.Documents = append(state.space.Documents, nil)
	copy(state.space.Documents[insertAt+1:], state.space.Documents[insertAt:])
	state.space.Documents[insertAt] = selected
	state.space.Select(insertAt)
	state.paneDocs[state.activePane] = selected
	state.paneTabAnchor[state.activePane] = nil
	state.tabActive = -1
}

func (state *model) paintViewTabMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(260), size.Width-parentRight-4)
	available := size.Height - menuHeight - statusHeight - 8
	rowHeight := float32(24)
	scrolling := float32(len(viewTabCommands))*rowHeight+8 > available
	visible := len(viewTabCommands)
	if scrolling {
		rowHeight = 21
		visible = max(1, int((available-52)/rowHeight))
	}
	state.viewTabVisible = visible
	state.viewTabOffset = min(max(0, state.viewTabOffset), len(viewTabCommands)-visible)
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
	canvas.HitWithOptions(viewTabPanelID, panel, "Tab commands", ui.HitOptions{Role: ui.RoleMenu})
	if scrolling {
		previous := ui.Rect{X: x + 4, Y: y + 4, Width: width - 8, Height: 22}
		next := ui.Rect{X: x + 4, Y: y + height - 26, Width: width - 8, Height: 22}
		canvas.DrawIcon(upIcon, ui.Rect{X: previous.X + previous.Width/2 - 8, Y: previous.Y + 3, Width: 16, Height: 16}, palette.muted)
		canvas.DrawIcon(downIcon, ui.Rect{X: next.X + next.Width/2 - 8, Y: next.Y + 3, Width: 16, Height: 16}, palette.muted)
		if hitButton(canvas, viewTabPreviousID, previous, "Previous tab commands") {
			state.viewTabOffset = max(0, state.viewTabOffset-1)
		}
		if hitButton(canvas, viewTabNextID, next, "Next tab commands") {
			state.viewTabOffset = min(len(viewTabCommands)-visible, state.viewTabOffset+1)
		}
	}
	selected := -1
	for row := range visible {
		index := state.viewTabOffset + row
		command := viewTabCommands[index]
		item := ui.Rect{X: x + 4, Y: y + 4 + controls/2 + float32(row)*rowHeight, Width: width - 8, Height: rowHeight}
		id := viewTabScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		checked := isTabColorCommand(command) && state.tabColors[state.paneDocument(state.activePane)] == tabColorCommand(command)
		left := float32(9)
		if isTabColorCommand(command) {
			state.tabColorMenuRow(canvas, item, command, checked)
			left = 28
		}
		smallLabel(canvas, command, ui.Rect{X: item.X + left, Y: item.Y, Width: item.Width - left - 9, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, ui.HitOptions{Role: ui.RoleMenuItem, Checked: checked})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(viewTabCommands[selected])
	}
}
