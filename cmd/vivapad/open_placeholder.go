package main

import "slices"

// openIntoWorkspace reconciles model state when Workspace replaces its only
// untitled tab with a file.
func (state *model) openIntoWorkspace(open func() error) error {
	if len(state.space.Documents) != 1 {
		return open()
	}
	previous := state.space.Current()
	peer := 1 - state.activePane
	keepPeer := state.splitEnabled && state.paneDocument(peer) == previous
	if err := open(); err != nil {
		return err
	}
	if len(state.space.Documents) != 1 || state.space.Current() == previous {
		return nil
	}
	if keepPeer {
		// The other pane still has a working tab. Put it back before any
		// input or Save can reach an editor outside the workspace.
		state.space.Documents = slices.Insert(state.space.Documents, 0, previous)
		state.space.Select(1)
		state.paneDocs[peer] = previous
		state.paneOwner[previous] = peer
		return nil
	}
	state.releaseDocument(previous)
	for pane, doc := range state.paneDocs {
		if doc == previous {
			state.paneDocs[pane] = nil
		}
	}
	return nil
}
