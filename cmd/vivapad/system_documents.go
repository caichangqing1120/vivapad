package main

func (state *model) queueSystemDocument(path string) {
	if path == "" || state.closingAccepted {
		return
	}
	state.pendingSystemDocuments = append(state.pendingSystemDocuments, path)
	state.invalidator.Invalidate()
}

func (state *model) consumeSystemDocuments() {
	// A document event can arrive while an in-app dialog owns focus or a
	// close decision still refers to the current workspace.
	if len(state.pendingSystemDocuments) == 0 || state.closingAccepted || state.batchActive ||
		state.editorCommandFocusBlocked() || state.markDialogOpen || state.byteRangeOpen {
		return
	}
	paths := state.pendingSystemDocuments
	state.pendingSystemDocuments = nil
	state.cancelFileCommandComposition(state.space.Current())
	if state.openDocumentPaths(paths) {
		state.requestNativeChooserFocus()
	}
}
