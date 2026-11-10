package main

import "vivapad/internal/document"

func (state *model) hasMultipleStreamSelections(doc *document.Document) bool {
	selected := state.multiSelections[state.activePane]
	return selected.active && selected.doc == doc && selected.value == doc.Editor.String() && len(selected.ranges) > 1 &&
		doc.Editor.Selection() == selected.ranges[len(selected.ranges)-1]
}

func (state *model) splitHasMultipleSelections(doc *document.Document) bool {
	if state.hasMultipleStreamSelections(doc) {
		return true
	}
	selected := state.rectangles[state.activePane]
	return selected.active && selected.doc == doc && selected.value == doc.Editor.String() &&
		len(doc.RectangleRows(state.rectangleForPane(state.activePane))) > 1
}

func isLineSortCommand(command string) bool {
	if _, _, ok := numericSortForCommand(command); ok {
		return true
	}
	switch command {
	case "Sort Lines Ascending", "Sort Lines Descending", "Sort Ascending (Ignore Case)", "Sort Descending (Ignore Case)",
		"Reverse Line Order", "Randomize Line Order", "Sort In Locale Order Ascending", "Sort In Locale Order Descending":
		return true
	}
	return false
}
