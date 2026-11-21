package main

import (
	"github.com/vivaui/viva/app"
	"vivapad/internal/external"
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

func (state *model) openCurrentConsole(kind external.ConsoleKind, label string) {
	if state.openConsole == nil {
		state.notice = label + " is unavailable on this platform"
		return
	}
	path := state.space.Current().Path
	if path == "" {
		state.notice = "Save the document before opening its folder"
		return
	}
	if err := state.openConsole(kind, path); err != nil {
		state.notice = label + ": " + err.Error()
		return
	}
	state.notice = "Opened " + label + " in containing folder"
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
