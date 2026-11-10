package main

import (
	"slices"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/session"
)

func (state *model) toggleSplit() {
	if state.splitEnabled {
		state.splitEnabled = false
		state.activePane = 0
		state.paneDocs[0], state.paneDocs[1] = state.space.Current(), nil
		return
	}
	current := state.space.Current()
	state.paneDocs = [2]*document.Document{current, current}
	state.activePane = 0
	for _, doc := range state.space.Documents {
		if state.paneOwner[doc] == 1 {
			state.paneDocs[1] = doc
			break
		}
	}
	if len(state.space.Documents) > 1 {
		if state.paneDocs[1] != current {
			if state.paneOwner[current] == 1 {
				state.activePane = 1
			}
		} else if state.space.Active > 0 {
			state.paneDocs[0] = state.space.Documents[state.space.Active-1]
			state.paneDocs[1] = current
			state.activePane = 1
		} else {
			state.paneDocs[1] = state.space.Documents[1]
		}
		state.paneOwner[state.paneDocs[1]] = 1
	}
	state.splitEnabled = true
	if state.zoomSync {
		size := state.paneFontSize(state.activePane)
		state.fontSize, state.subFontSize = size, size
	}
	if state.syncVertical || state.syncHorizontal {
		state.syncNeedsCapture = true
	}
}

func (state *model) resetPaneOwnership() {
	state.paneOwner = make(map[*document.Document]int, len(state.space.Documents))
	for _, doc := range state.space.Documents {
		state.paneOwner[doc] = 0
	}
}

func (state *model) applySessionView(view session.ViewState) {
	if !view.Split {
		return
	}
	for _, doc := range state.space.Documents {
		for _, path := range view.RightPaths {
			if doc.Path == path {
				state.paneOwner[doc] = 1
			}
		}
	}
	find := func(path string) *document.Document {
		for _, doc := range state.space.Documents {
			if path != "" && doc.Path == path {
				return doc
			}
		}
		return nil
	}
	left, right := find(view.Left), find(view.Right)
	if left == nil {
		for _, doc := range state.space.Documents {
			if state.paneOwner[doc] == 0 {
				left = doc
				break
			}
		}
	}
	if left == nil {
		left = state.space.Current()
	}
	if right == nil {
		for _, doc := range state.space.Documents {
			if state.paneOwner[doc] == 1 {
				right = doc
				break
			}
		}
	}
	if right == nil {
		right = left
	}
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane, state.splitRatio = true, view.ActivePane, view.Divider
	if index := state.documentIndex(state.paneDocs[state.activePane]); index >= 0 {
		state.space.Select(index)
	}
}

func (state *model) paneTabs(pane int) []*document.Document {
	previous := state.paneTabScratch[pane]
	docs := previous[:0]
	selected := state.paneDocument(pane)
	selectedFound := false
	for _, doc := range state.space.Documents {
		if state.paneOwner[doc] == pane {
			docs = append(docs, doc)
			selectedFound = selectedFound || doc == selected
		}
	}
	if !selectedFound && selected != nil {
		docs = append(docs, selected)
	}
	slices.SortStableFunc(docs, func(first, second *document.Document) int {
		if state.isPinned(first) == state.isPinned(second) {
			return 0
		}
		if state.isPinned(first) {
			return -1
		}
		return 1
	})
	for index := len(docs); index < len(previous); index++ {
		previous[index] = nil
	}
	state.paneTabScratch[pane] = docs
	return docs
}

func (state *model) paneDocument(pane int) *document.Document {
	if state.splitEnabled && pane >= 0 && pane < 2 && state.paneDocs[pane] != nil {
		return state.paneDocs[pane]
	}
	return state.space.Current()
}

func (state *model) codeEditorForPane(pane int) *components.CodeEditor {
	doc := state.paneDocument(pane)
	state.configureCode(doc)
	editors := state.editors
	if pane == 1 && state.splitEnabled {
		editors = state.secondEditors
	}
	editor := editors[doc]
	if editor == nil {
		editor = &components.CodeEditor{}
		editors[doc] = editor
	}
	return editor
}

func (state *model) activatePane(pane int) {
	if !state.splitEnabled || pane < 0 || pane > 1 {
		return
	}
	state.activePane = pane
	doc := state.paneDocument(pane)
	if selected := state.multiSelections[pane]; selected.active && selected.doc == doc && selected.value == doc.Editor.String() && len(selected.ranges) != 0 {
		doc.Editor.SetSelection(selected.ranges[len(selected.ranges)-1])
	}
	for index, candidate := range state.space.Documents {
		if candidate == doc {
			state.space.Select(index)
			return
		}
	}
}

func (state *model) showInActivePane(doc *document.Document) {
	if state.splitEnabled {
		state.paneDocs[state.activePane] = doc
		state.paneOwner[doc] = state.activePane
	} else {
		state.paneDocs[0] = doc
		state.paneOwner[doc] = 0
	}
}

func (state *model) rebindClosedDocument(closed *document.Document) {
	current := state.space.Current()
	for pane, doc := range state.paneDocs {
		if doc == closed {
			state.paneDocs[pane] = current
		}
	}
	if !state.splitEnabled || state.paneDocs[state.activePane] != current {
		state.showInActivePane(current)
	}
}

func (state *model) closePaneTab(pane int, doc *document.Document) {
	if !state.splitEnabled || state.paneDocs[1-pane] != doc {
		if index := state.documentIndex(doc); index >= 0 {
			state.closeTab(index)
		}
		return
	}
	if state.paneOwner[doc] == pane {
		state.paneOwner[doc] = 1 - pane
	}
	for _, candidate := range state.space.Documents {
		if candidate != doc && state.paneOwner[candidate] == pane {
			state.paneDocs[pane] = candidate
			if state.activePane == pane {
				state.activatePane(pane)
			}
			return
		}
	}
	state.toggleSplit()
}

func (state *model) moveToOtherView(clone bool) {
	if !state.splitEnabled {
		state.toggleSplit()
	}
	source, target := state.activePane, 1-state.activePane
	doc := state.paneDocument(source)
	state.paneDocs[target] = doc
	if !clone {
		state.paneOwner[doc] = target
		var fallback *document.Document
		for _, candidate := range state.space.Documents {
			if candidate != doc && state.paneOwner[candidate] == source {
				fallback = candidate
				break
			}
		}
		if fallback == nil {
			state.toggleSplit()
			return
		}
		state.paneDocs[source] = fallback
	}
	state.activePane = target
	if index := state.documentIndex(doc); index >= 0 {
		state.space.Select(index)
	}
}

func (state *model) cycleTabs(backward bool) {
	docs := state.space.Documents
	if state.splitEnabled {
		docs = state.paneTabs(state.activePane)
	}
	if len(docs) == 0 {
		return
	}
	selected := state.paneDocument(state.activePane)
	position := 0
	for index, doc := range docs {
		if doc == selected {
			position = index
			break
		}
	}
	if backward {
		position = (position - 1 + len(docs)) % len(docs)
	} else {
		position = (position + 1) % len(docs)
	}
	doc := docs[position]
	if index := state.documentIndex(doc); index >= 0 {
		state.space.Select(index)
		state.paneDocs[state.activePane] = doc
	}
}

func (state *model) replacePaneDocument(old, next *document.Document) {
	if state.isPinned(old) {
		state.pinned[next] = true
	}
	if owner, ok := state.paneOwner[old]; ok {
		state.paneOwner[next] = owner
	}
	for pane, doc := range state.paneDocs {
		if doc == old {
			state.paneDocs[pane] = next
		}
	}
}

func (state *model) splitPosition(width float32) float32 {
	minimum := min(200, max(70, (width-8)/2))
	return min(max(state.splitRatio*width, minimum), width-minimum-8)
}

func paneForTarget(target ui.ID) int {
	switch target {
	case editorID, scrollbarID, horizontalBarID, leftPaneID:
		return 0
	case rightEditorID, rightScrollbarID, rightHorizontalID, rightPaneID:
		return 1
	default:
		return -1
	}
}

func (state *model) splitKey(event ui.KeyEvent, width float32) ui.EventResult {
	if !state.splitEnabled || event.Target != splitterID {
		return ui.EventResult{}
	}
	position := state.splitPosition(width)
	switch event.Key {
	case ui.KeyLeft:
		position -= 16
	case ui.KeyRight:
		position += 16
	case ui.KeyHome:
		position = 0
	case ui.KeyEnd:
		position = width
	default:
		return ui.EventResult{}
	}
	minimum := min(200, max(70, (width-8)/2))
	state.splitRatio = min(max(position, minimum), width-minimum-8) / width
	return ui.EventResult{Handled: true, Repaint: true}
}
