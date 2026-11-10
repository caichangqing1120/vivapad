package main

import (
	"crypto/sha256"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

type multiSelectionTransition struct {
	doc           *document.Document
	before, after [32]byte
	beforeRanges  []text.Range
	afterRanges   []text.Range
}

func (state *model) recordMultiSelectionTransition(pane int, doc *document.Document, before, after string, previous multiSelectionState) {
	if before == after {
		return
	}
	entries := state.multiSelectionHistory[pane][:state.multiSelectionHistoryIndex[pane]]
	entries = append(entries, multiSelectionTransition{
		doc: doc, before: sha256.Sum256([]byte(before)), after: sha256.Sum256([]byte(after)),
		beforeRanges: append([]text.Range(nil), previous.ranges...),
		afterRanges:  append([]text.Range(nil), state.multiSelections[pane].ranges...),
	})
	if len(entries) > 128 {
		entries = append([]multiSelectionTransition(nil), entries[len(entries)-128:]...)
	}
	state.multiSelectionHistory[pane], state.multiSelectionHistoryIndex[pane] = entries, len(entries)
}

func (state *model) restoreMultiSelectionAcrossViews(doc *document.Document, before, after string, undo bool) {
	for _, pane := range []int{state.activePane, 1 - state.activePane} {
		if pane == 1 && !state.splitEnabled || state.paneDocument(pane) != doc {
			continue
		}
		entries, index := state.multiSelectionHistory[pane], state.multiSelectionHistoryIndex[pane]
		var transition multiSelectionTransition
		var ranges []text.Range
		if undo {
			if index == 0 {
				continue
			}
			transition = entries[index-1]
			if transition.doc != doc || transition.after != sha256.Sum256([]byte(before)) || transition.before != sha256.Sum256([]byte(after)) {
				continue
			}
			ranges = transition.beforeRanges
			state.multiSelectionHistoryIndex[pane]--
		} else {
			if index >= len(entries) {
				continue
			}
			transition = entries[index]
			if transition.doc != doc || transition.before != sha256.Sum256([]byte(before)) || transition.after != sha256.Sum256([]byte(after)) {
				continue
			}
			ranges = transition.afterRanges
			state.multiSelectionHistoryIndex[pane]++
		}
		if len(ranges) == 0 {
			state.multiSelections[pane] = multiSelectionState{}
		} else {
			state.multiSelections[pane] = multiSelectionState{
				doc: doc, value: after, ranges: append([]text.Range(nil), ranges...), active: true,
			}
			if pane == state.activePane {
				doc.Editor.SetSelection(ranges[len(ranges)-1])
			}
		}
		return
	}
}
