package main

import (
	"slices"

	"github.com/vivaui/viva/gfx"
)

type folderRowHit struct {
	path      string
	directory bool
	id        gfx.ID
}

func (state *model) folderHitsFor(rows []folderRow) []folderRowHit {
	hits := state.folderRowHits
	if len(hits) > len(rows) {
		clear(hits[len(rows):])
	}
	hits = slices.Grow(hits, max(0, len(rows)-len(hits)))[:len(rows)]
	for index, row := range rows {
		hit := &hits[index]
		if hit.path != row.path || hit.directory != row.directory {
			// A click in flight must not carry over to a new occupant of this slot.
			*hit = folderRowHit{path: row.path, directory: row.directory, id: folderRowScope.At(state.folderNextRowID)}
			state.folderNextRowID = (state.folderNextRowID + 1) & ((1 << 31) - 1)
		}
	}
	state.folderRowHits = hits
	return hits
}

func (state *model) paintFolderPanel(canvas *gfx.Canvas, bounds gfx.Rect) {
	canvas.Fill(bounds, gfx.RGB(250, 250, 250))
	canvas.Fill(gfx.Rect{X: bounds.Width - 1, Y: bounds.Y, Width: 1, Height: bounds.Height}, palette.border)
	canvas.Fill(gfx.Rect{Y: bounds.Y + 33, Width: bounds.Width, Height: 1}, palette.border)
	title := "Folder as Workspace"
	if bounds.Width < 210 {
		title = "Workspace"
	}
	smallLabel(canvas, title, gfx.Rect{X: 8, Y: bounds.Y + 5, Width: bounds.Width - 77, Height: 23}, palette.text, 11)
	for _, button := range []struct {
		id    gfx.ID
		icon  gfx.Icon
		label string
		x     float32
	}{
		{folderAddID, newIcon, "Add folder", bounds.Width - 69},
		{folderRefreshID, redoIcon, "Refresh folders", bounds.Width - 47},
		{folderCloseID, closeIcon, "Close Folder as Workspace", bounds.Width - 25},
	} {
		buttonBounds := gfx.Rect{X: button.x, Y: bounds.Y + 7, Width: 20, Height: 20}
		if canvas.Hovered() == button.id {
			canvas.Fill(buttonBounds, palette.hover)
		}
		canvas.DrawIcon(button.icon, buttonBounds.Inset(gfx.All(3)), palette.muted)
		if hitButton(canvas, button.id, buttonBounds, button.label) {
			switch button.id {
			case folderAddID:
				state.chooseFolderRoot()
			case folderRefreshID:
				state.refreshFolderTree()
			case folderCloseID:
				state.folderVisible = false
				state.stopFolderWatching()
				if state.functionListVisible {
					state.activeLeftDock = "functions"
				} else if state.documentListVisible {
					state.activeLeftDock = "documents"
				} else {
					state.activeLeftDock = ""
				}
				return
			}
		}
	}
	rowArea := gfx.Rect{Y: bounds.Y + 34, Width: bounds.Width - 1, Height: max(0, bounds.Height-34)}
	canvas.HitWithOptions(folderPanelID, rowArea, "Folder as Workspace", gfx.HitOptions{Role: gfx.RoleGroup})
	rows := state.folderRows()
	visible := max(1, int(rowArea.Height/22))
	focusPath := state.folderFocusPath
	state.folderFocusPath = ""
	if focusPath != "" {
		if index := slices.IndexFunc(rows, func(row folderRow) bool { return row.path == focusPath }); index >= 0 {
			state.folderOffset = min(max(state.folderOffset, index-visible+1), index)
		}
	}
	state.folderOffset = min(max(0, state.folderOffset), max(0, len(rows)-visible))
	visibleRows := rows[state.folderOffset:min(len(rows), state.folderOffset+visible)]
	hits := state.folderHitsFor(visibleRows)
	canvas.Clip(rowArea)
	var chosen *folderRow
	for index, row := range visibleRows {
		id := hits[index].id
		item := gfx.Rect{X: 2, Y: rowArea.Y + float32(index)*22, Width: rowArea.Width - 3, Height: 22}
		if row.path == state.folderSelected || canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		indent := min(float32(row.depth*13), bounds.Width-64)
		icon := fileIcon
		if row.directory {
			icon = rightIcon
			if state.folderExpanded[row.path] {
				icon = downIcon
			}
		}
		canvas.DrawIcon(icon, gfx.Rect{X: 7 + indent, Y: item.Y + 4, Width: 14, Height: 14}, palette.muted)
		textX := 26 + indent
		label := canvas.FitLine(row.name, 11, gfx.WeightRegular, max(20, item.Width-textX-4))
		smallLabel(canvas, label, gfx.Rect{X: textX, Y: item.Y, Width: max(20, item.Width-textX-4), Height: 22}, palette.text, 11)
		roleLabel := "Open " + row.name
		if row.directory {
			roleLabel = "Toggle folder " + row.name
		}
		canvas.HitWithOptions(id, item, roleLabel, gfx.HitOptions{Role: gfx.RoleButton})
		if row.path == focusPath {
			canvas.RequestFocus(id)
		}
		if canvas.Clicked(id) {
			chosen = &row
		}
	}
	canvas.Unclip()
	if chosen != nil && state.activateFolderRow(*chosen) {
		state.focusEditorOnTabClick(canvas)
	}
}
