package main

import (
	"slices"

	"github.com/vivaui/viva/gfx"
)

var bookmarkCommands = [...]string{
	"Toggle Bookmark", "Next Bookmark", "Previous Bookmark", "Clear All Bookmarks",
	"Cut Bookmarked Lines", "Copy Bookmarked Lines", "Paste to (Replace) Bookmarked Lines",
	"Remove Bookmarked Lines", "Remove Non-Bookmarked Lines", "Inverse Bookmarks",
}

func (state *model) bookmarkMenuKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	parent := menuItemScope.At(slices.Index(menuCommands[2], "Bookmark"))
	if state.menuOpen != 2 || event.Modifiers != 0 || event.Target != parent && event.Target.Scope() != bookmarkScope {
		return gfx.EventResult{}, false
	}
	if !state.bookmarkMenuOpen {
		if event.Target != parent || event.Key != gfx.KeyRight && event.Key != gfx.KeyEnter {
			return gfx.EventResult{}, false
		}
		state.command("Bookmark")
	}
	if event.Key == gfx.KeyLeft || event.Key == gfx.KeyEscape {
		state.bookmarkMenuOpen = false
		return gfx.EventResult{Handled: true, Repaint: true, Focus: parent, FocusVisible: true}, true
	}
	readOnly := state.isReadOnly(state.space.Current())
	index, step := -1, 1
	for i := range bookmarkCommands {
		if event.Target == bookmarkScope.At(i) {
			index = i
			break
		}
	}
	switch event.Key {
	case gfx.KeyEnter:
		if index >= 0 && (!readOnly || !state.textMutation(bookmarkCommands[index])) {
			state.command(bookmarkCommands[index])
			state.focusViewRequested, state.focusViewArmed = false, false
			return gfx.EventResult{Handled: true, Repaint: true, Focus: state.commandEditorFocusID()}, true
		}
		index = -1
	case gfx.KeyRight:
		if index >= 0 {
			return gfx.EventResult{Handled: true}, true
		}
	case gfx.KeyHome, gfx.KeyPageUp:
		index = -1
	case gfx.KeyEnd, gfx.KeyPageDown:
		index, step = len(bookmarkCommands), -1
	case gfx.KeyUp:
		index, step = max(0, index), -1
	case gfx.KeyDown:
	default:
		return gfx.EventResult{}, false
	}
	for range bookmarkCommands {
		index = (index + step + len(bookmarkCommands)) % len(bookmarkCommands)
		if !readOnly || !state.textMutation(bookmarkCommands[index]) {
			return gfx.EventResult{Handled: true, Repaint: true, Focus: bookmarkScope.At(index), FocusVisible: true}, true
		}
	}
	return gfx.EventResult{Handled: true}, true
}

func (state *model) paintBookmarkMenu(canvas *gfx.Canvas, size gfx.Size, parentRight, parentY float32) {
	palette := state.palette()
	width := min(float32(320), size.Width-parentRight-4)
	height := float32(len(bookmarkCommands))*27 + 8
	x := parentRight
	y := min(parentY, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.popup)
	readOnly := state.isReadOnly(state.space.Current())
	hot := canvas.Hovered()
	if focus := canvas.FocusVisible(); focus.Scope() == bookmarkScope {
		hot = focus
	}
	selected := -1
	for index, command := range bookmarkCommands {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := bookmarkScope.At(index)
		disabled := readOnly && state.textMutation(command)
		if hot == id && !disabled {
			canvas.Fill(item, palette.hover)
		}
		color := palette.text
		if disabled {
			color = palette.muted
		}
		label := canvas.FitLine(command, 11, gfx.WeightRegular, item.Width-18)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, color, 11)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem, Disabled: disabled})
		if !disabled && canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(bookmarkCommands[selected])
	}
}
