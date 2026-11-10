package main

import (
	"math/rand/v2"
	"time"

	"vivapad/internal/document"
)

func (state *model) sortRectangleCommand(doc *document.Document, command string) bool {
	pane := state.activePane
	previous := state.rectangles[pane]
	if !previous.active || previous.doc != doc || previous.value != doc.Editor.String() {
		return false
	}
	if command == "Sort In Locale Order Ascending" || command == "Sort In Locale Order Descending" {
		return state.sortLocaleRectangleCommand(doc, command, previous)
	}
	mode, ascending, matched := numericSortForCommand(command)
	if !matched {
		ascending = true
		switch command {
		case "Sort Lines Ascending", "Sort Lines Descending":
			mode, ascending = document.SortLexicographic, command == "Sort Lines Ascending"
		case "Sort Ascending (Ignore Case)", "Sort Descending (Ignore Case)":
			mode, ascending = document.SortIgnoreCase, command == "Sort Ascending (Ignore Case)"
		case "Reverse Line Order":
			mode = document.SortReverse
		case "Randomize Line Order":
			mode = document.SortRandom
		default:
			return false
		}
	}
	if previous.composing {
		state.notice = "Finish column composition before sorting"
		return true
	}
	var random *rand.Rand
	if mode == document.SortRandom {
		random = rand.New(rand.NewPCG(uint64(time.Now().Unix()), 1))
	}
	changed := false
	var sortError error
	state.editLineTargetsWithBookmarks(doc, "replace", func() {
		changed, sortError = doc.SortRectangle(state.rectangleForPane(pane), mode, ascending, random)
	})
	if sortError != nil {
		state.notice = sortError.Error()
	} else if changed {
		after := doc.Editor.String()
		caret := doc.Editor.Caret()
		state.rectangles[pane] = rectangleSelection{doc: doc, anchor: caret, caret: caret, value: after, active: true}
		state.recordRectangleTransition(pane, doc, previous.value, after, previous)
		state.notice = command
	} else {
		state.notice = "Line order unchanged"
	}
	return true
}
