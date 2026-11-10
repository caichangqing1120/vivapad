package main

import "vivapad/internal/document"

func (state *model) editLineTargetsWithBookmarks(doc *document.Document, _ string, edit func()) {
	state.editWithBookmarkAnchor(doc, edit)
}
