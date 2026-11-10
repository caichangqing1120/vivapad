package main

import (
	"strconv"

	"vivapad/internal/document"
)

type closeTarget struct {
	doc  *document.Document
	pane int
}

func (state *model) startBatchClose(keep *document.Document) {
	targets := make([]closeTarget, 0, len(state.space.Documents))
	for _, doc := range state.space.Documents {
		if doc != keep {
			targets = append(targets, closeTarget{doc: doc, pane: -1})
		}
	}
	state.startBatchTargets(targets, keep)
}

func (state *model) startViewClose(command string) {
	pane := state.activePane
	docs := state.space.Documents
	selected := state.space.Current()
	if state.splitEnabled {
		docs = state.paneTabs(pane)
		selected = state.paneDocument(pane)
	}
	selectedIndex := -1
	for index, doc := range docs {
		if doc == selected {
			selectedIndex = index
			break
		}
	}
	targets := make([]closeTarget, 0, len(docs))
	for index := len(docs) - 1; index >= 0; index-- {
		include := false
		switch command {
		case "Close All to the Left":
			include = index < selectedIndex
		case "Close All to the Right":
			include = index > selectedIndex
		case "Close All Unchanged":
			include = !docs[index].Dirty()
		}
		if include {
			targets = append(targets, closeTarget{doc: docs[index], pane: pane})
		}
	}
	if command == "Close All Unchanged" {
		selected = nil
	}
	state.startBatchTargets(targets, selected)
}

func (state *model) startAllButPinned() {
	var targets []closeTarget
	panes := 1
	if state.splitEnabled {
		panes = 2
	}
	for pane := range panes {
		docs := state.space.Documents
		if state.splitEnabled {
			docs = state.paneTabs(pane)
		}
		for index := len(docs) - 1; index >= 0; index-- {
			if !state.isPinned(docs[index]) {
				targets = append(targets, closeTarget{doc: docs[index], pane: pane})
			}
		}
	}
	state.startBatchTargets(targets, nil)
}

func (state *model) startBatchTargets(targets []closeTarget, keep *document.Document) {
	if state.pendingClose >= 0 || state.pendingReload >= 0 || state.replaceConfirmOpen || state.filesDialogOpen || state.windowsDialogOpen || state.renameDoc != nil || state.renamePlan != nil {
		state.notice = "Finish the open dialog before closing tabs"
		return
	}
	state.batchQueue = append(state.batchQueue[:0], targets...)
	if len(state.batchQueue) == 0 {
		state.notice = "No matching tabs to close"
		return
	}
	state.batchActive, state.batchKeep, state.batchClosed = true, keep, 0
	state.continueBatchClose()
}

func (state *model) continueBatchClose() {
	for len(state.batchQueue) > 0 {
		target := state.batchQueue[0]
		state.batchQueue[0] = closeTarget{}
		state.batchQueue = state.batchQueue[1:]
		index := state.documentIndex(target.doc)
		if index < 0 {
			continue
		}
		if target.pane >= 0 && state.splitEnabled && state.paneDocs[1-target.pane] == target.doc {
			state.closePaneTab(target.pane, target.doc)
			state.batchClosed++
			continue
		}
		if target.doc.Dirty() {
			state.pendingClose = index
			return
		}
		state.closeTab(index)
		if state.pendingClose >= 0 {
			return
		}
		state.batchClosed++
	}
	keep, closed := state.batchKeep, state.batchClosed
	state.batchActive, state.batchQueue, state.batchKeep, state.batchClosed = false, nil, nil, 0
	if index := state.documentIndex(keep); index >= 0 {
		state.space.Select(index)
		if !state.splitEnabled || state.paneDocs[state.activePane] != keep {
			state.showInActivePane(keep)
		}
	}
	state.notice = "Closed " + strconv.Itoa(closed) + " tab(s)"
}
