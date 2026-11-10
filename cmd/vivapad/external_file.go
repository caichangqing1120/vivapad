package main

import (
	"github.com/vivaui/viva/app"
)

func (state *model) openDefaultViewer() {
	path := state.space.Current().Path
	if path == "" {
		state.notice = "Save the document before opening it in the default viewer"
		return
	}
	if err := app.OpenPath(path); err != nil {
		state.notice = "Default viewer: " + err.Error()
		return
	}
	state.notice = "Opened in default viewer"
}

func (state *model) revealCurrentFile() {
	path := state.space.Current().Path
	if path == "" {
		state.notice = "Save the document before opening its folder"
		return
	}
	if err := state.revealPath(path); err != nil {
		state.notice = "Containing folder: " + err.Error()
		return
	}
	state.notice = "Opened containing folder"
}
