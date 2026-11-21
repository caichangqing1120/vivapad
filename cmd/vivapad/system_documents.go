package main

func (state *model) queueSystemDocument(path string) {
	if path == "" || state.closingAccepted {
		return
	}
	state.pendingSystemDocuments = append(state.pendingSystemDocuments, path)
	state.invalidator.Invalidate()
}

func (state *model) consumeSystemDocuments() {
	// Let a pending dialog or line jump finish on the current document first.
	if len(state.pendingSystemDocuments) == 0 || state.closingAccepted || state.batchActive ||
		state.editorCommandFocusBlocked() || state.goToOpen || state.markDialogOpen || state.byteRangeOpen {
		return
	}
	paths := state.pendingSystemDocuments
	state.pendingSystemDocuments = nil
	state.cancelFileCommandComposition(state.space.Current())
	if state.openDocumentPaths(paths) {
		state.requestNativeChooserFocus()
	}
}
