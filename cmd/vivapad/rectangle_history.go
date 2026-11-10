package main

import (
	"crypto/sha256"
	"strings"

	"vivapad/internal/document"
)

type rectangleSnapshot struct {
	anchor, caret               int
	anchorVirtual, caretVirtual int
	pending, active             bool
}

type rectangleTransition struct {
	doc                   *document.Document
	before, after         [32]byte
	beforeRect, afterRect rectangleSnapshot
}

func snapshotRectangle(selection rectangleSelection) rectangleSnapshot {
	return rectangleSnapshot{
		anchor: selection.anchor, caret: selection.caret,
		anchorVirtual: selection.anchorVirtual, caretVirtual: selection.caretVirtual,
		pending: selection.pending, active: selection.active,
	}
}

func (state *model) recordRectangleTransition(pane int, doc *document.Document, before, after string, previous rectangleSelection) {
	if before == after {
		return
	}
	entries := state.rectangleHistory[pane][:state.rectangleHistoryIndex[pane]]
	entries = append(entries, rectangleTransition{
		doc: doc, before: sha256.Sum256([]byte(before)), after: sha256.Sum256([]byte(after)),
		beforeRect: snapshotRectangle(previous), afterRect: snapshotRectangle(state.rectangles[pane]),
	})
	if len(entries) > 128 {
		entries = append([]rectangleTransition(nil), entries[len(entries)-128:]...)
	}
	state.rectangleHistory[pane], state.rectangleHistoryIndex[pane] = entries, len(entries)
	state.relocateSharedRectangle(pane, doc, before, after)
}

func (state *model) restoreRectangleAcrossViews(doc *document.Document, before, after string, undo bool) {
	owner := state.activePane
	if !state.restoreRectangleTransition(owner, doc, before, after, undo) {
		other := 1 - owner
		if state.splitEnabled && state.paneDocument(other) == doc && state.restoreRectangleTransition(other, doc, before, after, undo) {
			owner = other
		}
	}
	state.relocateSharedRectangle(owner, doc, before, after)
}

func (state *model) relocateSharedRectangle(source int, doc *document.Document, before, after string) {
	if !state.splitEnabled || state.paneDocument(1-source) != doc {
		return
	}
	other := &state.rectangles[1-source]
	if other.doc != doc || other.value != before || !other.active && !other.pending {
		return
	}
	oldLines, newLines := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(oldLines) != len(newLines) {
		*other = rectangleSelection{}
		return
	}
	anchor, okAnchor := mapRectanglePosition(oldLines, newLines, other.anchor)
	caret, okCaret := mapRectanglePosition(oldLines, newLines, other.caret)
	if !okAnchor || !okCaret {
		*other = rectangleSelection{}
		return
	}
	other.anchor, other.caret, other.value = anchor, caret, after
	other.copyText, other.copyValid = "", false
}

func mapRectanglePosition(before, after []string, index int) (int, bool) {
	oldStart, newStart := 0, 0
	for line := range before {
		oldRunes, newRunes := []rune(before[line]), []rune(after[line])
		oldEnd := oldStart + len(oldRunes)
		if index <= oldEnd {
			local := index - oldStart
			prefix := 0
			for prefix < len(oldRunes) && prefix < len(newRunes) && oldRunes[prefix] == newRunes[prefix] {
				prefix++
			}
			suffix := 0
			for suffix < len(oldRunes)-prefix && suffix < len(newRunes)-prefix && oldRunes[len(oldRunes)-suffix-1] == newRunes[len(newRunes)-suffix-1] {
				suffix++
			}
			oldChangeEnd, newChangeEnd := len(oldRunes)-suffix, len(newRunes)-suffix
			switch {
			case local < prefix:
			case local > oldChangeEnd:
				local += len(newRunes) - len(oldRunes)
			default:
				local = newChangeEnd
			}
			return newStart + local, true
		}
		oldStart, newStart = oldEnd+1, newStart+len(newRunes)+1
	}
	return 0, false
}

func (state *model) restoreRectangleTransition(pane int, doc *document.Document, before, after string, undo bool) bool {
	entries, index := state.rectangleHistory[pane], state.rectangleHistoryIndex[pane]
	var transition rectangleTransition
	var restored rectangleSnapshot
	if undo {
		if index == 0 {
			return false
		}
		transition = entries[index-1]
		if transition.doc != doc || transition.after != sha256.Sum256([]byte(before)) || transition.before != sha256.Sum256([]byte(after)) {
			return false
		}
		restored = transition.beforeRect
		state.rectangleHistoryIndex[pane]--
	} else {
		if index >= len(entries) {
			return false
		}
		transition = entries[index]
		if transition.doc != doc || transition.before != sha256.Sum256([]byte(before)) || transition.after != sha256.Sum256([]byte(after)) {
			return false
		}
		restored = transition.afterRect
		state.rectangleHistoryIndex[pane]++
	}
	if !restored.active && !restored.pending {
		state.rectangles[pane] = rectangleSelection{}
		return true
	}
	state.rectangles[pane] = rectangleSelection{
		doc: doc, anchor: restored.anchor, caret: restored.caret,
		anchorVirtual: restored.anchorVirtual, caretVirtual: restored.caretVirtual,
		pending: restored.pending, active: restored.active, value: after,
	}
	if pane == state.activePane {
		doc.Editor.MoveTo(restored.caret, false)
	}
	return true
}
