package main

import (
	"path/filepath"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/recent"
)

func (state *model) rememberRecent(doc *document.Document) {
	if doc == nil || doc.Path == "" {
		return
	}
	state.forgetClosed(doc.Path)
	encoding := document.UTF8
	if doc.SavedEncoding().IsLegacy() {
		encoding = doc.SavedEncoding()
	}
	state.recentEntries = recent.Add(state.recentEntries, recent.Entry{Path: doc.Path, Encoding: encoding})
	if state.recentPath != "" {
		if err := recent.Save(state.recentPath, state.recentEntries); err != nil {
			state.notice = "Recent files not saved: " + err.Error()
		}
	}
}

func (state *model) openRecent(index int) {
	state.menuOpen, state.recentOpen = -1, false
	if index < 0 || index >= len(state.recentEntries) {
		return
	}
	entry := state.recentEntries[index]
	if err := state.space.OpenRecorded(entry.Path, entry.Encoding); err != nil {
		state.notice = "Cannot open recent " + filepath.Base(entry.Path) + ": " + err.Error()
		return
	}
	state.showInActivePane(state.space.Current())
	state.rememberRecent(state.space.Current())
}

func (state *model) clearRecent() {
	if state.closedPath != "" {
		if err := recent.Clear(state.closedPath); err != nil {
			state.notice = "Closed-file history not cleared: " + err.Error()
			return
		}
	}
	if state.recentPath != "" {
		if err := recent.Clear(state.recentPath); err != nil {
			state.notice = "Recent files not cleared: " + err.Error()
			return
		}
	}
	state.recentEntries = nil
	state.closedHistory = nil
	state.menuOpen, state.recentOpen = -1, false
	state.notice = "Recent files cleared"
}

func (state *model) paintRecentMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	rows := len(state.recentEntries)
	if rows == 0 {
		rows = 1
	}
	if len(state.closedHistory) != 0 {
		rows++
	}
	if len(state.recentEntries) != 0 || len(state.closedHistory) != 0 {
		rows++
	}
	width := min(float32(380), size.Width-parentRight-4)
	height := float32(rows)*27 + 8
	x := parentRight
	y := min(menuHeight+4+10*fileMenuRowHeight, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, entry := range state.recentEntries {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := recentScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := filepath.Base(entry.Path) + "   " + filepath.Dir(entry.Path)
		label = canvas.FitLine(label, 11, gfx.WeightRegular, item.Width-18)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, entry.Path, gfx.HitOptions{Role: gfx.RoleMenuItem, Hint: entry.Path})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	row := len(state.recentEntries)
	if len(state.recentEntries) == 0 {
		smallLabel(canvas, "No recent files", gfx.Rect{X: x + 14, Y: y + 4, Width: width - 28, Height: 27}, palette.muted, 11)
		row++
	}
	if len(state.closedHistory) != 0 {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(row)*27, Width: width - 8, Height: 27}
		if canvas.Hovered() == recentRestoreID {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, "Restore Recent Closed File", gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: 27}, palette.text, 11)
		canvas.HitWithOptions(recentRestoreID, item, "Restore Recent Closed File", gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(recentRestoreID) {
			state.restoreClosed()
		}
		row++
	}
	if len(state.recentEntries) != 0 || len(state.closedHistory) != 0 {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(row)*27, Width: width - 8, Height: 27}
		if canvas.Hovered() == recentClearID {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, "Clear Recent Files", gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: 27}, palette.text, 11)
		if hitButton(canvas, recentClearID, item, "Clear Recent Files") {
			state.clearRecent()
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.openRecent(selected)
	}
}
