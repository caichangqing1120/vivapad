package main

import (
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

type selectAnchor struct {
	doc   *document.Document
	index int
	value string
}

func (state *model) beginEndSelect() {
	doc := state.space.Current()
	state.multiSelections[state.activePane] = multiSelectionState{}
	state.rectangles[state.activePane] = rectangleSelection{}
	anchor := &state.selectAnchors[state.activePane]
	if anchor.doc != doc {
		*anchor = selectAnchor{doc: doc, index: doc.Editor.Caret(), value: doc.Editor.String()}
		state.notice = "Selection start set"
		return
	}
	state.relocateSelectAnchors(doc, anchor.value, doc.Editor.String(), -1)
	doc.Editor.SetSelection(text.Range{Start: anchor.index, End: doc.Editor.Caret()})
	*anchor = selectAnchor{}
	state.notice = "Selection completed"
}

func (state *model) relocateSelectAnchors(doc *document.Document, before, after string, editStart int) {
	for pane := range state.selectAnchors {
		anchor := &state.selectAnchors[pane]
		if anchor.doc != doc {
			continue
		}
		if anchor.value != before {
			adjustSelectAnchor(anchor, anchor.value, before, -1)
		}
		adjustSelectAnchor(anchor, before, after, editStart)
	}
}

func adjustSelectAnchor(anchor *selectAnchor, before, after string, editStart int) {
	if before == after {
		anchor.value = after
		return
	}
	oldRunes, newRunes := []rune(before), []rune(after)
	prefix := 0
	for prefix < len(oldRunes) && prefix < len(newRunes) && (editStart < 0 || prefix < editStart) && oldRunes[prefix] == newRunes[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldRunes)-prefix && suffix < len(newRunes)-prefix && oldRunes[len(oldRunes)-suffix-1] == newRunes[len(newRunes)-suffix-1] {
		suffix++
	}
	oldEnd := len(oldRunes) - suffix
	switch {
	case anchor.index <= prefix:
	case anchor.index >= oldEnd:
		anchor.index += len(newRunes) - len(oldRunes)
	default:
		anchor.index = prefix
	}
	anchor.index = min(max(anchor.index, 0), len(newRunes))
	anchor.value = after
}
