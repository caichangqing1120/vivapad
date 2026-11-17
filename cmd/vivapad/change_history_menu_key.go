package main

import (
	"slices"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func (state *model) changeHistoryMenuKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if state.menuOpen != 2 || event.Modifiers != 0 {
		return gfx.EventResult{}, false
	}
	parent := menuItemScope.At(slices.Index(menuCommands[2], "Change History"))
	if event.Target != parent && event.Target.Scope() != changeHistoryScope {
		return gfx.EventResult{}, false
	}
	submenu, horizontal := components.SubmenuCommandForKey(event.Key)
	if !state.changeHistoryMenuOpen {
		if event.Target != parent || event.Key != gfx.KeyEnter && (!horizontal || submenu != components.SubmenuCommandOpen) {
			return gfx.EventResult{}, false
		}
		state.command("Change History")
	}
	if event.Key == gfx.KeyEscape || horizontal && submenu == components.SubmenuCommandClose {
		state.changeHistoryMenuOpen = false
		return gfx.EventResult{Handled: true, Repaint: true, Focus: parent, FocusVisible: true}, true
	}
	unavailable := false
	if doc := state.space.Current(); doc != nil {
		_, unavailable = changeHistoryUnavailable(doc)
	}
	enabled := func(index int) bool {
		return !unavailable || changeHistoryItems[index].command == "Clear Change History"
	}
	current := -1
	for index := range changeHistoryItems {
		if event.Target == changeHistoryScope.At(index) {
			current = index
		}
	}
	if event.Key == gfx.KeyEnter && current >= 0 && enabled(current) {
		state.command(changeHistoryItems[current].command)
		focus := editorID
		if state.splitEnabled && state.activePane == 1 {
			focus = rightEditorID
		}
		// Keyboard dispatch can restore focus now; paint dispatch needs the
		// deferred request because its editor has already been drawn.
		state.focusViewRequested, state.focusViewArmed = false, false
		return gfx.EventResult{Handled: true, Repaint: true, Focus: focus}, true
	}
	start, step := current, 1
	switch {
	case event.Key == gfx.KeyHome || event.Key == gfx.KeyPageUp || event.Key == gfx.KeyEnter:
		start = -1
	case event.Key == gfx.KeyEnd || event.Key == gfx.KeyPageDown:
		start, step = len(changeHistoryItems), -1
	case event.Key == gfx.KeyUp:
		step = -1
		if start < 0 {
			start = 0
		}
	case event.Key == gfx.KeyDown:
	case horizontal && submenu == components.SubmenuCommandOpen:
		if current >= 0 && enabled(current) {
			return gfx.EventResult{Handled: true, Repaint: true, Focus: event.Target, FocusVisible: true}, true
		}
		start = -1
	default:
		return gfx.EventResult{}, false
	}
	for range len(changeHistoryItems) {
		start = (start + step + len(changeHistoryItems)) % len(changeHistoryItems)
		if enabled(start) {
			return gfx.EventResult{Handled: true, Repaint: true, Focus: changeHistoryScope.At(start), FocusVisible: true}, true
		}
	}
	return gfx.EventResult{Handled: true}, true
}

func (state *model) changeHistoryMenuPointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	if state.menuOpen != 2 || !state.changeHistoryMenuOpen || event.Kind != gfx.PointerMove {
		return gfx.EventResult{}, false
	}
	for index, item := range changeHistoryItems {
		if event.Target != changeHistoryScope.At(index) {
			continue
		}
		if doc := state.space.Current(); doc != nil {
			if _, unavailable := changeHistoryUnavailable(doc); unavailable && item.command != "Clear Change History" {
				return gfx.EventResult{Handled: true}, true
			}
		}
		return gfx.EventResult{Handled: true, Repaint: true, Focus: event.Target}, true
	}
	return gfx.EventResult{}, false
}
