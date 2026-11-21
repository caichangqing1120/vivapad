package main

import (
	"path/filepath"
	"slices"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/recent"
)

const recentFilesCommand = "Recent Files"

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

func isRecentMenuItem(id gfx.ID) bool {
	return id.Scope() == recentScope || id == recentRestoreID || id == recentClearID
}

func (state *model) recentMenuKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if state.menuOpen != 0 || event.Modifiers != 0 {
		return gfx.EventResult{}, false
	}
	parent := menuItemScope.At(slices.Index(menuCommands[0], recentFilesCommand))
	if event.Target != parent && (!state.recentOpen || !isRecentMenuItem(event.Target)) {
		return gfx.EventResult{}, false
	}
	command, horizontal := components.SubmenuCommandForKey(event.Key)
	closing := event.Key == gfx.KeyEscape || horizontal && command == components.SubmenuCommandClose
	switch event.Key {
	case gfx.KeyEnter, gfx.KeyDown, gfx.KeyUp, gfx.KeyHome, gfx.KeyEnd, gfx.KeyPageUp, gfx.KeyPageDown:
	default:
		if !horizontal && !closing || closing && !state.recentOpen {
			return gfx.EventResult{}, false
		}
	}

	focus := func(id gfx.ID) (gfx.EventResult, bool) {
		return gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}, true
	}
	if closing {
		state.recentOpen = false
		return focus(parent)
	}
	if !state.recentOpen {
		state.command(recentFilesCommand)
	}
	var items []gfx.ID
	for i := range state.recentEntries {
		items = append(items, recentScope.At(i))
	}
	if len(state.closedHistory) > 0 {
		items = append(items, recentRestoreID)
	}
	if len(items) == 0 {
		return focus(parent)
	}
	items = append(items, recentClearID)
	index := slices.Index(items, event.Target)
	if event.Key == gfx.KeyEnter && index >= 0 {
		switch event.Target {
		case recentRestoreID:
			state.restoreClosed()
		case recentClearID:
			state.clearRecent()
		default:
			state.openRecent(index)
		}
		if state.menuOpen >= 0 {
			return focus(event.Target)
		}
		state.focusViewRequested, state.focusViewArmed = false, false
		return gfx.EventResult{Handled: true, Repaint: true, Focus: state.commandEditorFocusID()}, true
	}

	switch event.Key {
	case gfx.KeyDown:
		index = (index + 1) % len(items)
	case gfx.KeyUp:
		index = (max(0, index) + len(items) - 1) % len(items)
	case gfx.KeyEnd, gfx.KeyPageDown:
		index = len(items) - 1
	case gfx.KeyHome, gfx.KeyPageUp, gfx.KeyEnter:
		index = 0
	default:
		index = max(0, index)
	}
	return focus(items[index])
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
	hot := canvas.Hovered()
	if visible := canvas.FocusVisible(); isRecentMenuItem(visible) {
		hot = visible
	}
	selected := -1
	for index, entry := range state.recentEntries {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := recentScope.At(index)
		if hot == id {
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
	restore := false
	if len(state.closedHistory) != 0 {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(row)*27, Width: width - 8, Height: 27}
		if hot == recentRestoreID {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, "Restore Recent Closed File", gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: 27}, palette.text, 11)
		canvas.HitWithOptions(recentRestoreID, item, "Restore Recent Closed File", gfx.HitOptions{Role: gfx.RoleMenuItem})
		restore = canvas.Clicked(recentRestoreID)
		row++
	}
	clear := false
	if len(state.recentEntries) != 0 || len(state.closedHistory) != 0 {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(row)*27, Width: width - 8, Height: 27}
		if hot == recentClearID {
			canvas.Fill(item, palette.hover)
		}
		smallLabel(canvas, "Clear Recent Files", gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: 27}, palette.text, 11)
		canvas.HitWithOptions(recentClearID, item, "Clear Recent Files", gfx.HitOptions{Role: gfx.RoleMenuItem})
		clear = canvas.Clicked(recentClearID)
	}
	canvas.PopLayer()
	switch {
	case clear:
		state.clearRecent()
	case restore:
		state.restoreClosed()
	case selected >= 0:
		state.openRecent(selected)
	default:
		return
	}
	state.focusEditorOnTabClick(canvas)
}
