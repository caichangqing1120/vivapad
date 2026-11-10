package main

import (
	"path/filepath"

	"vivapad/internal/document"
	"vivapad/internal/recent"
)

func (state *model) recordClosed(doc *document.Document) {
	if doc == nil || doc.Path == "" {
		return
	}
	encoding := document.UTF8
	if doc.SavedEncoding().IsLegacy() {
		encoding = doc.SavedEncoding()
	}
	state.closedHistory = recent.Add(state.closedHistory, recent.Entry{Path: doc.Path, Encoding: encoding})
	if err := state.saveClosedHistory(); err != nil {
		state.notice = "Closed-file history not saved: " + err.Error()
	}
}

func (state *model) saveClosedHistory() error {
	if state.closedPath == "" {
		return nil
	}
	if len(state.closedHistory) == 0 {
		return recent.Clear(state.closedPath)
	}
	return recent.Save(state.closedPath, state.closedHistory)
}

func (state *model) forgetClosed(path string) {
	for index, entry := range state.closedHistory {
		if entry.Path != path {
			continue
		}
		copy(state.closedHistory[index:], state.closedHistory[index+1:])
		state.closedHistory[len(state.closedHistory)-1] = recent.Entry{}
		state.closedHistory = state.closedHistory[:len(state.closedHistory)-1]
		if err := state.saveClosedHistory(); err != nil {
			state.notice = "Closed-file history not updated: " + err.Error()
		}
		return
	}
}

func (state *model) restoreClosed() {
	state.cancelMacroForCommand()
	state.menuOpen, state.recentOpen = -1, false
	if len(state.closedHistory) == 0 {
		state.notice = "No recently closed file"
		return
	}
	entry := state.closedHistory[0]
	var err error
	if entry.Encoding.IsLegacy() {
		err = state.space.OpenWithEncoding(entry.Path, entry.Encoding)
	} else {
		err = state.space.Open(entry.Path)
	}
	if err != nil {
		state.notice = "Cannot restore " + filepath.Base(entry.Path) + ": " + err.Error()
		return
	}
	state.showInActivePane(state.space.Current())
	state.closedHistory[0] = recent.Entry{}
	state.closedHistory = state.closedHistory[1:]
	state.notice = "Restored " + filepath.Base(entry.Path)
	if err := state.saveClosedHistory(); err != nil {
		state.notice = "Restored file, but closed-file history was not saved: " + err.Error()
	}
	state.rememberRecent(state.space.Current())
}
