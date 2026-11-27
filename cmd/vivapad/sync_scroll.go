package main

import "vivapad/internal/document"

func (state *model) captureSyncScroll() {
	if !state.splitEnabled || state.paneDocument(0).LargeFile() != nil || state.paneDocument(1).LargeFile() != nil {
		state.syncNeedsCapture = true
		return
	}
	main, sub := state.codeEditorForPane(0), state.codeEditorForPane(1)
	state.syncScrollOffsetY = main.ScrollY() - sub.ScrollY()
	state.syncScrollOffsetX = main.ScrollX() - sub.ScrollX()
	state.syncScrollSeenY = [2]float32{main.ScrollY(), sub.ScrollY()}
	state.syncScrollSeenX = [2]float32{main.ScrollX(), sub.ScrollX()}
	state.syncScrollDocs = [2]*document.Document{state.paneDocument(0), state.paneDocument(1)}
	state.syncNeedsCapture = false
}

func (state *model) syncScrollFrom(pane int) {
	if state.distractionFree || !state.splitEnabled || pane < 0 || pane > 1 || !state.syncVertical && !state.syncHorizontal {
		return
	}
	if state.paneDocument(0).LargeFile() != nil || state.paneDocument(1).LargeFile() != nil {
		state.syncNeedsCapture = true
		return
	}
	source, peer := state.codeEditorForPane(pane), state.codeEditorForPane(1-pane)
	sign := float32(1)
	if pane == 0 {
		sign = -1
	}
	if state.syncVertical {
		peer.SetScrollY(source.ScrollY() + sign*state.syncScrollOffsetY)
	}
	if state.syncHorizontal {
		peer.SetScrollX(source.ScrollX() + sign*state.syncScrollOffsetX)
	}
	main, sub := state.codeEditorForPane(0), state.codeEditorForPane(1)
	state.syncScrollSeenY = [2]float32{main.ScrollY(), sub.ScrollY()}
	state.syncScrollSeenX = [2]float32{main.ScrollX(), sub.ScrollX()}
	state.syncScrollDocs = [2]*document.Document{state.paneDocument(0), state.paneDocument(1)}
}

func (state *model) syncScrollAfterPaint() {
	if !state.splitEnabled || !state.syncVertical && !state.syncHorizontal {
		return
	}
	if state.paneDocument(0).LargeFile() != nil || state.paneDocument(1).LargeFile() != nil {
		state.syncNeedsCapture = true
		return
	}
	if state.syncNeedsCapture {
		state.captureSyncScroll()
		return
	}
	main, sub := state.codeEditorForPane(0), state.codeEditorForPane(1)
	vertical := [2]float32{main.ScrollY(), sub.ScrollY()}
	horizontal := [2]float32{main.ScrollX(), sub.ScrollX()}
	changedDocs := state.syncScrollDocs != [2]*document.Document{state.paneDocument(0), state.paneDocument(1)}
	choose := func(current, previous [2]float32) int {
		if !changedDocs {
			if current[0] != previous[0] && current[1] == previous[1] {
				return 0
			}
			if current[1] != previous[1] && current[0] == previous[0] {
				return 1
			}
		}
		return state.activePane
	}
	if state.syncVertical && (changedDocs || vertical != state.syncScrollSeenY) {
		if choose(vertical, state.syncScrollSeenY) == 0 {
			sub.SetScrollY(main.ScrollY() - state.syncScrollOffsetY)
		} else {
			main.SetScrollY(sub.ScrollY() + state.syncScrollOffsetY)
		}
	}
	if state.syncHorizontal && (changedDocs || horizontal != state.syncScrollSeenX) {
		if choose(horizontal, state.syncScrollSeenX) == 0 {
			sub.SetScrollX(main.ScrollX() - state.syncScrollOffsetX)
		} else {
			main.SetScrollX(sub.ScrollX() + state.syncScrollOffsetX)
		}
	}
	if main.ScrollY() != vertical[0] || sub.ScrollY() != vertical[1] || main.ScrollX() != horizontal[0] || sub.ScrollX() != horizontal[1] {
		state.invalidator.Invalidate()
	}
	state.syncScrollSeenY = [2]float32{main.ScrollY(), sub.ScrollY()}
	state.syncScrollSeenX = [2]float32{main.ScrollX(), sub.ScrollX()}
	state.syncScrollDocs = [2]*document.Document{state.paneDocument(0), state.paneDocument(1)}
}
