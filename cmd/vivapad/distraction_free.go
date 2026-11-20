package main

const distractionFreeCommand = "Distraction Free Mode"

func (state *model) toggleDistractionFree() {
	if state.requestFullscreen == nil {
		state.notice = "Distraction Free Mode unavailable"
		return
	}
	if state.distractionFreeEntering || state.fullscreen && !state.distractionFree {
		return
	}
	enter := !state.distractionFree
	state.distractionFreeEntering = enter
	if err := state.requestFullscreen(enter); err != nil {
		state.distractionFreeEntering = false
		state.notice = "Distraction Free Mode unavailable: " + err.Error()
	}
}

func (state *model) observeFullscreen(fullscreen bool) {
	wasDistractionFree := state.distractionFree
	state.fullscreen = fullscreen
	state.distractionFree = fullscreen && (state.distractionFree || state.distractionFreeEntering)
	state.distractionFreeEntering = false
	state.menuOpen, state.menuOverflowOpen = -1, false
	if wasDistractionFree != state.distractionFree {
		state.syncNeedsCapture = true
		state.requestNativeChooserFocus()
	}
	state.invalidator.Invalidate()
}

func (state *model) fullscreenExitCommand() string {
	if state.distractionFree {
		return distractionFreeCommand
	}
	return "Toggle Full Screen Mode"
}
