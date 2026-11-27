package main

import (
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/gfx"
)

var autoCompletionCommands = [...]string{
	functionCompletionCommand, "Word Completion", parameterHintCommand,
	previousHintCommand, nextHintCommand, "Path Completion",
}

func (state *model) paintAutoCompletionMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	palette := state.palette()
	width := min(float32(275), size.Width-parentRight-4)
	height := float32(len(autoCompletionCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+17*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.popup)
	selected := -1
	for index, command := range autoCompletionCommands {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := autoCompletionScope.At(index)
		disabled := (command == previousHintCommand || command == nextHintCommand) &&
			(state.calltip == nil || len(state.calltip.call.Overloads) < 2)
		if !disabled && canvas.Hovered() == id {
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
		state.command(autoCompletionCommands[selected])
	}
}

func (state *model) paintWordCompletionPopup(canvas *gfx.Canvas, size gfx.Size) {
	palette := state.palette()
	if !state.validWordCompletion() {
		state.wordCompletion = nil
		if state.wordFocusPending {
			focus := editorID
			if state.activePane == 1 {
				focus = rightEditorID
			}
			canvas.RequestFocus(focus)
			state.wordFocusPending = false
		}
		return
	}
	current := state.wordCompletion
	editor := state.codeEditorForPane(current.pane)
	caret, ok := editor.CaretBounds()
	if !ok {
		editor.Reveal(current.doc.Editor.ByteOffset(current.doc.Editor.Caret()))
		state.invalidator.Invalidate()
		id := editorID
		if current.pane == 1 {
			id = rightEditorID
		}
		for _, hit := range canvas.Hits() {
			if hit.ID == id {
				caret = gfx.Rect{X: hit.Bounds.X + hit.Bounds.Width - 12, Y: hit.Bounds.Y + 8, Width: 1, Height: 20}
				break
			}
		}
	}
	visible := min(7, len(current.items))
	current.offset = min(max(0, current.offset), len(current.items)-visible)
	if current.selected < current.offset {
		current.offset = current.selected
	} else if current.selected >= current.offset+visible {
		current.offset = current.selected - visible + 1
	}
	width := min(float32(245), size.Width-8)
	height := float32(visible)*22 + 8
	x := min(max(4, caret.X), size.Width-width-4)
	y := caret.Y + caret.Height + 2
	if y+height > size.Height-statusHeight-2 {
		y = max(menuHeight+toolbarHeight+tabHeight, caret.Y-height-2)
	}
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.popup)
	chosen := -1
	for index := current.offset; index < current.offset+visible; index++ {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index-current.offset)*22, Width: width - 8, Height: 22}
		id := wordCompletionScope.At(index)
		if index == current.selected || canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		value := current.items[index]
		if current.kind == completionPath {
			directory := strings.HasSuffix(value, string(filepath.Separator))
			value = filepath.Base(strings.TrimSuffix(value, string(filepath.Separator)))
			if directory {
				value += string(filepath.Separator)
			}
		}
		label := canvas.FitLine(value, 11, gfx.WeightRegular, item.Width-16)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 8, Y: item.Y, Width: item.Width - 16, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, current.items[index], gfx.HitOptions{Role: gfx.RoleOption, Selected: index == current.selected, Hint: current.items[index]})
		if canvas.Clicked(id) {
			chosen = index
		}
	}
	canvas.PopLayer()
	if chosen >= 0 {
		state.acceptWordCompletion(chosen)
		state.wordFocusPending = true
	} else if state.wordFocusPending {
		focus := editorID
		if current.pane == 1 {
			focus = rightEditorID
		}
		canvas.RequestFocus(focus)
		state.wordFocusPending = false
	}
}
