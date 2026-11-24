package main

import (
	"errors"

	"github.com/vivaui/viva/gfx"
)

const postItCommand = "Post-It"

type postItMode struct {
	active  bool
	pending bool
	// Track effects separately so a failed restoration can be retried.
	raised    bool
	frameless bool
	failure   string
}

func (state *model) togglePostIt() {
	mode := &state.postIt
	if mode.pending || state.distractionFree || state.distractionFreeEntering {
		return
	}
	if state.requestDecoration == nil || state.requestAlwaysOnTop == nil {
		state.notice = "Post-It unavailable"
		return
	}
	mode.pending, mode.failure = true, ""
	if mode.active || mode.raised {
		state.leavePostIt()
		return
	}
	if !state.alwaysOnTop {
		if err := state.requestAlwaysOnTop(true); err != nil {
			state.finishPostIt(false, err)
			return
		}
		mode.raised = true
	}
	if state.fullscreen {
		state.finishPostIt(true, nil)
		return
	}
	state.changePostItDecoration(false, func(err error) {
		if err != nil {
			state.finishPostIt(false, errors.Join(err, state.restorePostItLevel()))
			return
		}
		mode.frameless = true
		state.finishPostIt(true, nil)
	})
}

func (state *model) leavePostIt() {
	mode := &state.postIt
	wasRaised := mode.raised
	if err := state.restorePostItLevel(); err != nil {
		state.finishPostIt(mode.active, err)
		return
	}
	if !mode.frameless {
		state.finishPostIt(false, nil)
		return
	}
	state.changePostItDecoration(true, func(err error) {
		if err != nil {
			if wasRaised {
				rollback := state.requestAlwaysOnTop(true)
				mode.raised = rollback == nil
				err = errors.Join(err, rollback)
			}
			state.finishPostIt(true, err)
			return
		}
		mode.frameless = false
		state.finishPostIt(false, nil)
	})
}

func (state *model) restorePostItLevel() error {
	if !state.postIt.raised {
		return nil
	}
	if err := state.requestAlwaysOnTop(false); err != nil {
		return err
	}
	state.postIt.raised = false
	return nil
}

func (state *model) changePostItDecoration(decorated bool, completed func(error)) {
	if err := state.requestDecoration(decorated, completed); err != nil {
		completed(err)
	}
}

func (state *model) finishPostIt(active bool, err error) {
	state.postIt.active, state.postIt.pending = active, false
	if err != nil {
		state.postIt.failure = "Post-It: " + err.Error()
	}
	state.notice = state.postIt.failure
	state.requestNativeChooserFocus()
	state.invalidator.Invalidate()
}

func (state *model) postItShortcut(event gfx.KeyEvent) gfx.EventResult {
	if event.Key != gfx.KeyF12 || event.Modifiers != 0 || event.Release ||
		state.requestDecoration == nil || state.distractionFree || state.editorCommandFocusBlocked() && state.menuOpen < 0 {
		return gfx.EventResult{}
	}
	if !event.Repeat {
		state.command(postItCommand)
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}
