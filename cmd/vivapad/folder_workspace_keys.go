package main

import (
	"path/filepath"
	"slices"

	"github.com/vivaui/viva/gfx"
)

func (state *model) folderWorkspaceKey(event gfx.KeyEvent) gfx.EventResult {
	if !state.folderVisible || state.activeDock() != "workspace" || event.Target.Scope() != folderRowScope || event.Modifiers != 0 {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyUp, gfx.KeyDown, gfx.KeyHome, gfx.KeyEnd, gfx.KeyLeft, gfx.KeyRight, gfx.KeyEnter:
	default:
		return gfx.EventResult{}
	}
	if event.Release || event.Key == gfx.KeyEnter && event.Repeat {
		return gfx.EventResult{Handled: true}
	}
	result := gfx.EventResult{Handled: true, Repaint: true}
	hitIndex := slices.IndexFunc(state.folderRowHits, func(hit folderRowHit) bool { return hit.id == event.Target })
	if hitIndex < 0 {
		return result
	}
	hit := state.folderRowHits[hitIndex]
	state.consumeFolderChanges()
	rows := state.folderRows()
	index := slices.IndexFunc(rows, func(row folderRow) bool { return row.path == hit.path && row.directory == hit.directory })
	if index < 0 {
		return result
	}
	row := rows[index]
	switch event.Key {
	case gfx.KeyUp:
		index = max(0, index-1)
	case gfx.KeyDown:
		index = min(len(rows)-1, index+1)
	case gfx.KeyHome:
		index = 0
	case gfx.KeyEnd:
		index = len(rows) - 1
	case gfx.KeyRight:
		if row.directory {
			if !state.folderExpanded[row.path] {
				state.activateFolderRow(row)
			} else if index+1 < len(rows) && rows[index+1].depth > row.depth {
				index++
			}
		}
	case gfx.KeyLeft:
		if row.directory && state.folderExpanded[row.path] {
			state.activateFolderRow(row)
		} else if row.depth > 0 {
			parent := filepath.Dir(row.path)
			if at := slices.IndexFunc(rows[:index], func(candidate folderRow) bool { return candidate.path == parent }); at >= 0 {
				index = at
			}
		}
	case gfx.KeyEnter:
		if state.activateFolderRow(row) {
			state.focusViewRequested, state.focusViewArmed = false, false
			result.Focus = state.commandEditorFocusID()
			return result
		}
	}
	state.folderSelected, state.folderFocusPath = rows[index].path, rows[index].path
	return result
}
