package main

import "vivapad/internal/document"

func (state *model) isPinned(doc *document.Document) bool {
	return state.pinned[doc]
}

func (state *model) restoreSessionPinned(flags map[string]bool) {
	for _, doc := range state.space.Documents {
		if flags[doc.Path] {
			state.pinned[doc] = true
		}
	}
}

func (state *model) togglePinned(doc *document.Document, pane int) {
	if state.documentIndex(doc) < 0 {
		return
	}
	var boundary *document.Document
	for _, candidate := range state.paneTabs(pane) {
		if candidate != doc && !state.isPinned(candidate) {
			boundary = candidate
			break
		}
	}
	oldIndex := state.documentIndex(doc)
	state.space.Documents = append(state.space.Documents[:oldIndex], state.space.Documents[oldIndex+1:]...)
	insertAt := len(state.space.Documents)
	for index, candidate := range state.space.Documents {
		if candidate == boundary {
			insertAt = index
			break
		}
	}
	state.space.Documents = append(state.space.Documents, nil)
	copy(state.space.Documents[insertAt+1:], state.space.Documents[insertAt:])
	state.space.Documents[insertAt] = doc
	if state.isPinned(doc) {
		delete(state.pinned, doc)
	} else {
		state.pinned[doc] = true
	}
	state.space.Select(insertAt)
	state.activePane = pane
	state.paneDocs[pane] = doc
	state.paneTabAnchor[pane] = nil
}
