package main

func (state *model) paneFontSize(pane int) int {
	if state.splitEnabled && pane == 1 {
		return state.subFontSize
	}
	return state.fontSize
}

func (state *model) zoom(command string) {
	pane := state.activePane
	if !state.splitEnabled {
		pane = 0
	}
	size := state.paneFontSize(pane)
	switch command {
	case "Zoom In":
		size = min(24, size+1)
	case "Zoom Out":
		size = max(9, size-1)
	case "Restore Default Zoom":
		size = 13
	}
	if pane == 1 || state.zoomSync {
		state.subFontSize = size
	}
	if pane == 0 || state.zoomSync {
		state.fontSize = size
	}
	state.savePreferences()
}

func (state *model) toggleZoomSync() {
	state.zoomSync = !state.zoomSync
	if state.zoomSync {
		size := state.paneFontSize(state.activePane)
		state.fontSize, state.subFontSize = size, size
	}
	state.savePreferences()
}
