package main

import (
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func (state *model) paintSplitTabs(canvas *ui.Canvas, width, origin float32) {
	position := state.splitPosition(width)
	y := menuHeight + toolbarHeight
	if state.fullscreen {
		y = 0
	}
	state.paintPaneTabs(canvas, 0, ui.Rect{X: origin, Y: y, Width: position, Height: tabHeight})
	divider := ui.Rect{X: origin + position, Y: y, Width: 8, Height: tabHeight}
	canvas.Fill(divider, palette.tabs)
	canvas.Fill(ui.Rect{X: divider.X + 3, Y: y, Width: 1, Height: tabHeight}, palette.border)
	rightWidth := width - position - 8
	if state.fullscreen {
		rightWidth -= 30
	}
	state.paintPaneTabs(canvas, 1, ui.Rect{X: origin + position + 8, Y: y, Width: rightWidth, Height: tabHeight})
}

func (state *model) paintPaneTabs(canvas *ui.Canvas, pane int, bounds ui.Rect) {
	canvas.Fill(bounds, palette.tabs)
	canvas.Fill(ui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height - 1, Width: bounds.Width, Height: 1}, palette.border)
	paneID := leftPaneID
	tabScope, closeScope := leftTabScope, leftCloseScope
	backID, nextID := leftTabsBackID, leftTabsNextID
	pinScope := leftPinScope
	if pane == 1 {
		paneID, tabScope, closeScope = rightPaneID, rightTabScope, rightCloseScope
		backID, nextID = rightTabsBackID, rightTabsNextID
		pinScope = rightPinScope
	}
	canvas.HitWithOptions(paneID, bounds, "Activate editor view", ui.HitOptions{Role: ui.RoleGroup})
	if canvas.Clicked(paneID) {
		state.activatePane(pane)
	}
	docs := state.paneTabs(pane)
	tabWidth := min(float32(157), max(90, bounds.Width-16))
	visible := max(1, int((bounds.Width-10)/(tabWidth+4)))
	overflow := len(docs) > visible
	if overflow {
		tabWidth = min(157, max(90, bounds.Width-68))
		visible = max(1, int((bounds.Width-58)/(tabWidth+4)))
	}
	selected := state.paneDocument(pane)
	selectedIndex := 0
	for index, doc := range docs {
		if doc == selected {
			selectedIndex = index
			break
		}
	}
	if state.paneTabAnchor[pane] != selected || state.paneTabVisible[pane] != visible || state.paneTabIndex[pane] != selectedIndex {
		if selectedIndex < state.paneTabOffset[pane] {
			state.paneTabOffset[pane] = selectedIndex
		} else if selectedIndex >= state.paneTabOffset[pane]+visible {
			state.paneTabOffset[pane] = selectedIndex - visible + 1
		}
		state.paneTabAnchor[pane] = selected
		state.paneTabVisible[pane], state.paneTabIndex[pane] = visible, selectedIndex
	}
	state.paneTabOffset[pane] = min(max(0, state.paneTabOffset[pane]), max(0, len(docs)-visible))
	chosen, closing := -1, -1
	var pinning *document.Document
	for local := state.paneTabOffset[pane]; local < len(docs) && local-state.paneTabOffset[pane] < visible; local++ {
		doc := docs[local]
		global := state.documentIndex(doc)
		if global < 0 {
			continue
		}
		x := bounds.X + 7 + float32(local-state.paneTabOffset[pane])*(tabWidth+4)
		rect := ui.Rect{X: x, Y: bounds.Y + 2, Width: tabWidth, Height: tabHeight - 2}
		background, accent := state.tabColorPaint(doc, doc == selected)
		canvas.Border(rect, 1, palette.border, background)
		if doc == selected && pane == state.activePane {
			canvas.Fill(ui.Rect{X: x + 1, Y: bounds.Y + 1, Width: tabWidth - 2, Height: 2}, accent)
		}
		name := doc.Name
		if doc.Dirty() {
			name = "* " + name
		}
		name = canvas.FitLine(name, 11, ui.WeightRegular, tabWidth-56)
		smallLabel(canvas, name, ui.Rect{X: x + 10, Y: bounds.Y + 2, Width: tabWidth - 56, Height: 24}, palette.text, 11)
		id := tabScope.At(global)
		canvas.HitWithOptions(id, rect, doc.Name, ui.HitOptions{Role: ui.RoleTab, Selected: doc == selected})
		if canvas.Clicked(id) {
			chosen = global
		}
		pinRect := ui.Rect{X: x + tabWidth - 42, Y: bounds.Y + 5, Width: 17, Height: 18}
		if canvas.Hovered() == pinScope.At(global) {
			canvas.Fill(pinRect, palette.hover)
		}
		pinColor, pinLabel := palette.muted, "Pin "+doc.Name
		if state.isPinned(doc) {
			pinColor, pinLabel = ui.RGB(8, 105, 87), "Unpin "+doc.Name
		}
		canvas.DrawIcon(pinIcon, controlIconBounds(pinRect), pinColor)
		if hitButton(canvas, pinScope.At(global), pinRect, pinLabel) {
			pinning = doc
		}
		closeRect := ui.Rect{X: x + tabWidth - 22, Y: bounds.Y + 5, Width: 17, Height: 18}
		if canvas.Hovered() == closeScope.At(global) {
			canvas.Fill(closeRect, palette.hover)
		}
		canvas.DrawIcon(closeIcon, controlIconBounds(closeRect), palette.muted)
		if hitButton(canvas, closeScope.At(global), closeRect, "Close "+doc.Name) {
			closing = global
		}
	}
	if chosen >= 0 {
		state.activePane = pane
		state.space.Select(chosen)
		state.paneDocs[pane] = state.space.Current()
	}
	if closing >= 0 {
		state.closePaneTab(pane, state.space.Documents[closing])
	}
	if pinning != nil {
		state.togglePinned(pinning, pane)
	}
	if overflow {
		left := ui.Rect{X: bounds.X + bounds.Width - 49, Y: bounds.Y + 3, Width: 22, Height: 22}
		right := ui.Rect{X: bounds.X + bounds.Width - 25, Y: bounds.Y + 3, Width: 22, Height: 22}
		canvas.Border(left, 1, palette.border, palette.toolbar)
		canvas.Border(right, 1, palette.border, palette.toolbar)
		canvas.DrawIcon(leftIcon, controlIconBounds(left), palette.text)
		canvas.DrawIcon(rightIcon, controlIconBounds(right), palette.text)
		if hitButton(canvas, backID, left, "Previous view tabs") {
			state.paneTabOffset[pane] = max(0, state.paneTabOffset[pane]-1)
		}
		if hitButton(canvas, nextID, right, "Next view tabs") {
			state.paneTabOffset[pane] = min(len(docs)-visible, state.paneTabOffset[pane]+1)
		}
	}
}

func (state *model) documentIndex(doc *document.Document) int {
	for index, candidate := range state.space.Documents {
		if candidate == doc {
			return index
		}
	}
	return -1
}
