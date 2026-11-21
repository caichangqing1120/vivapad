package main

import "slices"

func (state *model) searchSubmenuParent() string {
	switch {
	case state.bookmarkMenuOpen:
		return "Bookmark"
	case state.changeHistoryMenuOpen:
		return "Change History"
	case state.markJumpMenuOpen:
		if state.markJumpPrevious {
			return "Jump Up"
		}
		return "Jump Down"
	case state.selectedSearchMenuOpen:
		return "Selected Text Search"
	}
	switch state.styleMenuMode {
	case "all":
		return "Style All Occurrences of Token"
	case "one":
		return "Style One Token"
	case "clear":
		return "Clear Style"
	case "copy":
		return "Copy Styled Text"
	}
	return ""
}

func (state *model) revealSearchSubmenuParent() {
	index := slices.Index(menuCommands[2], state.searchSubmenuParent())
	if index < 0 || state.searchMenuVisible <= 0 {
		return
	}
	if index < state.searchMenuOffset {
		state.searchMenuOffset = index
	} else if index >= state.searchMenuOffset+state.searchMenuVisible {
		state.searchMenuOffset = index - state.searchMenuVisible + 1
	}
}

func (state *model) closeSearchSubmenus() {
	state.bookmarkMenuOpen, state.changeHistoryMenuOpen = false, false
	state.markJumpMenuOpen, state.selectedSearchMenuOpen = false, false
	state.styleMenuMode = ""
}
