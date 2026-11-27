package main

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

type documentListRow struct {
	doc     *document.Document
	pane    int
	heading string
}

func (state *model) leftDockCount() int {
	count := 0
	if state.folderVisible {
		count++
	}
	if state.functionListVisible {
		count++
	}
	if state.documentListVisible {
		count++
	}
	if state.characterVisible {
		count++
	}
	return count
}

func (state *model) openDocumentRows() []documentListRow {
	rows := state.documentListRows[:0]
	for pane := 0; pane < 2; pane++ {
		if pane == 1 && !state.splitEnabled {
			break
		}
		heading := "Main View"
		if pane == 1 {
			heading = "Sub View"
		}
		rows = append(rows, documentListRow{pane: pane, heading: heading})
		var docs []*document.Document
		if state.splitEnabled {
			docs = state.paneTabs(pane)
		} else {
			docs = state.space.Documents
		}
		for _, doc := range docs {
			rows = append(rows, documentListRow{doc: doc, pane: pane})
		}
	}
	state.documentListRows = rows
	return rows
}

func (state *model) documentListRowIndex(row documentListRow) int {
	ids := state.documentListRowIDs[row.pane]
	if index, ok := ids[row.doc]; ok {
		return index
	}
	if ids == nil {
		ids = make(map[*document.Document]int)
		state.documentListRowIDs[row.pane] = ids
	}
	// A reordered row or a clone in the other view must not inherit a press.
	index := state.documentListNextRowID
	state.documentListNextRowID = (index + 1) & ((1 << 31) - 1)
	ids[row.doc] = index
	return index
}

func (state *model) paintDocumentList(canvas *gfx.Canvas, bounds gfx.Rect) {
	palette := state.palette()
	canvas.Fill(bounds, palette.dock)
	canvas.Fill(gfx.Rect{X: bounds.Width - 1, Y: bounds.Y, Width: 1, Height: bounds.Height}, palette.border)
	canvas.Fill(gfx.Rect{Y: bounds.Y + 33, Width: bounds.Width, Height: 1}, palette.border)
	smallLabel(canvas, "Document List", gfx.Rect{X: 9, Y: bounds.Y + 3, Width: bounds.Width - 39, Height: 27}, palette.text, 11)
	closeBounds := gfx.Rect{X: bounds.Width - 26, Y: bounds.Y + 7, Width: 18, Height: 18}
	canvas.DrawIcon(closeIcon, closeBounds.Inset(gfx.All(3)), palette.muted)
	if hitButton(canvas, documentListCloseID, closeBounds, "Close Document List") {
		state.documentListVisible = false
		state.activeLeftDock = state.activeDock()
		return
	}

	rows := state.openDocumentRows()
	available := max(0, bounds.Height-34)
	scrolling := float32(len(rows))*23 > available
	controlHeight := float32(0)
	if scrolling {
		controlHeight = 22
	}
	visible := max(1, int(max(0, available-2*controlHeight)/23))
	state.documentListVisibleRows = visible
	if state.documentListFocusRequested || state.documentListAnchor != state.paneDocument(state.activePane) || state.documentListAnchorPane != state.activePane {
		for index, row := range rows {
			if row.doc == state.paneDocument(state.activePane) && row.pane == state.activePane {
				if index < state.documentListOffset {
					state.documentListOffset = index
				} else if index >= state.documentListOffset+visible {
					state.documentListOffset = index - visible + 1
				}
				break
			}
		}
		state.documentListAnchor, state.documentListAnchorPane = state.paneDocument(state.activePane), state.activePane
		state.documentListFocusRequested = false
	}
	state.documentListOffset = min(max(0, state.documentListOffset), max(0, len(rows)-visible))
	if scrolling {
		for _, control := range []struct {
			id    gfx.ID
			icon  gfx.Icon
			label string
			y     float32
			step  int
		}{
			{documentListPreviousID, upIcon, "Previous documents", bounds.Y + 34, -1},
			{documentListNextID, downIcon, "Next documents", bounds.Y + bounds.Height - 22, 1},
		} {
			button := gfx.Rect{X: 1, Y: control.y, Width: bounds.Width - 2, Height: 22}
			canvas.DrawIcon(control.icon, gfx.Rect{X: button.X + button.Width/2 - 8, Y: button.Y + 3, Width: 16, Height: 16}, palette.muted)
			if hitButton(canvas, control.id, button, control.label) {
				state.documentListOffset = min(max(0, state.documentListOffset+control.step), max(0, len(rows)-visible))
			}
		}
	}
	rowArea := gfx.Rect{X: 1, Y: bounds.Y + 34 + controlHeight, Width: bounds.Width - 2, Height: max(0, available-2*controlHeight)}
	canvas.HitWithOptions(documentListPanelID, rowArea, "Open documents", gfx.HitOptions{Role: gfx.RoleGroup})
	canvas.Clip(rowArea)
	var activating, closing *documentListRow
	for index := state.documentListOffset; index < len(rows) && index-state.documentListOffset < visible; index++ {
		row := rows[index]
		item := gfx.Rect{X: 3, Y: rowArea.Y + float32(index-state.documentListOffset)*23, Width: rowArea.Width - 5, Height: min(23, rowArea.Y+rowArea.Height-(rowArea.Y+float32(index-state.documentListOffset)*23))}
		if row.heading != "" {
			canvas.Fill(item, palette.tabs)
			smallLabel(canvas, row.heading, gfx.Rect{X: item.X + 7, Y: item.Y, Width: item.Width - 14, Height: item.Height}, palette.muted, 10)
			continue
		}
		rowIndex := state.documentListRowIndex(row)
		rowID := documentListRowScope.At(rowIndex)
		closeID := documentListRowCloseScope.At(rowIndex)
		selected := row.doc == state.paneDocument(row.pane) && (!state.splitEnabled || row.pane == state.activePane)
		if selected || canvas.Hovered() == rowID || canvas.Hovered() == closeID {
			canvas.Fill(item, palette.hover)
		}
		if selected {
			_, accent := state.tabColorPaint(row.doc, true)
			canvas.Fill(gfx.Rect{X: item.X, Y: item.Y, Width: 2, Height: item.Height}, accent)
		} else if color := state.tabColors[row.doc]; color >= 1 && color <= 5 {
			canvas.Fill(gfx.Rect{X: item.X, Y: item.Y, Width: 2, Height: item.Height}, tabColorPastels[color-1])
		}
		name := row.doc.Name
		if row.doc.Dirty() {
			name = "* " + name
		}
		if state.isPinned(row.doc) {
			canvas.DrawIcon(pinIcon, gfx.Rect{X: item.X + 6, Y: item.Y + 5, Width: 13, Height: 13}, palette.accent)
		}
		nameX := item.X + 8
		if state.isPinned(row.doc) {
			nameX += 16
		}
		name = canvas.FitLine(name, 11, gfx.WeightRegular, max(12, item.X+item.Width-nameX-27))
		smallLabel(canvas, name, gfx.Rect{X: nameX, Y: item.Y, Width: max(12, item.X+item.Width-nameX-27), Height: item.Height}, palette.text, 11)
		hint := []string{row.doc.Path}
		if row.doc.Dirty() {
			hint = append(hint, "Modified")
		}
		if state.isPinned(row.doc) {
			hint = append(hint, "Pinned")
		}
		if color := state.tabColorHint(row.doc); color != "" {
			hint = append(hint, color)
		}
		label := "Open " + row.doc.Name
		if state.splitEnabled && row.pane == 1 && state.paneDocs[0] == row.doc {
			label += " from sub view"
		}
		canvas.HitWithOptions(rowID, item, label, gfx.HitOptions{Role: gfx.RoleButton, Selected: selected, Hint: strings.Join(hint, ", ")})
		if canvas.Clicked(rowID) {
			activating = &row
		}
		closeBounds := gfx.Rect{X: item.X + item.Width - 23, Y: item.Y + 3, Width: 19, Height: 18}
		canvas.DrawIcon(closeIcon, closeBounds.Inset(gfx.All(3)), palette.muted)
		closeLabel := "Close " + row.doc.Name
		if state.splitEnabled && row.pane == 1 && state.paneDocs[0] == row.doc {
			closeLabel += " from sub view"
		}
		if hitButton(canvas, closeID, closeBounds, closeLabel) {
			closing = &row
		}
	}
	canvas.Unclip()
	if activating != nil {
		if index := state.documentIndex(activating.doc); index >= 0 {
			changed := state.space.Current() != activating.doc || state.activePane != activating.pane
			if state.splitEnabled {
				state.paneDocs[activating.pane] = activating.doc
				state.activatePane(activating.pane)
			} else {
				state.space.Select(index)
				state.paneDocs[0] = activating.doc
			}
			if changed {
				state.focusEditorOnTabClick(canvas)
			}
		}
	}
	if closing != nil {
		state.closePaneTab(closing.pane, closing.doc)
	}
}
