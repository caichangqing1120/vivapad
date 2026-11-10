package main

import (
	"fmt"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func (state *model) openIncrementalSearch(repeat bool) {
	if state.incrementOpen {
		if repeat {
			state.findIncremental(false, true)
		}
		state.incrementFocusPending = true
		return
	}
	state.searchOpen, state.goToOpen, state.incrementOpen = false, false, true
	if selected := state.space.Current().SelectedSearchText(false); selected != "" {
		state.incrementQuery = text.NewEditor(selected)
	}
	state.incrementQuery.SetSelection(text.Range{Start: 0, End: state.incrementQuery.RuneCount()})
	state.incrementFocusPending = true
	state.findIncremental(false, false)
}

func (state *model) closeIncrementalSearch() {
	state.incrementOpen = false
	state.incrementMatches = nil
	state.incrementStatus = ""
	state.incrementFocusPending = false
	state.incrementDocument = nil
	state.incrementValue = ""
}

func (state *model) incrementalTextInput() *app.TextInput {
	return &app.TextInput{State: app.TextStateOf(state.incrementQuery), OnEvent: func(event app.TextEvent) (app.TextState, error) {
		if event.Kind == app.TextCommandEvent && (event.Command == app.TextInsertNewline || event.Command == app.TextInsertLineBreak) {
			state.findIncremental(event.Command == app.TextInsertLineBreak, true)
			return app.TextStateOf(state.incrementQuery), nil
		}
		event.Apply(state.incrementQuery)
		if event.Kind != app.TextSetSelection {
			if _, composing := state.incrementQuery.Composition(); !composing {
				state.findIncremental(false, false)
			}
		}
		return app.TextStateOf(state.incrementQuery), nil
	}}
}

func (state *model) findIncremental(backward, advance bool) {
	doc := state.space.Current()
	state.incrementDocument, state.incrementValue = doc, doc.Editor.String()
	query := state.incrementQuery.String()
	if query == "" {
		state.incrementStatus = ""
		state.incrementMatches = nil
		return
	}
	found, wrapped := doc.FindIncremental(query, state.incrementMatchCase, backward, advance)
	state.refreshIncrementMatches()
	if !found {
		state.incrementStatus = "Phrase not found"
		return
	}
	state.incrementStatus = state.incrementCountStatus(doc)
	if wrapped {
		if backward {
			state.incrementStatus += " Reached top, continued"
		} else {
			state.incrementStatus += " Reached end, continued"
		}
	}
}

func (state *model) refreshIncrementMatches() {
	if !state.incrementOpen || state.incrementQuery.String() == "" || !state.incrementCount && !state.incrementHighlight {
		state.incrementMatches = nil
		return
	}
	state.incrementMatches = state.space.Current().IncrementalMatches(state.incrementQuery.String(), state.incrementMatchCase)
}

func (state *model) incrementCountStatus(doc *document.Document) string {
	if !state.incrementCount {
		return ""
	}
	selection := doc.Editor.Selection()
	start, end := doc.Editor.ByteOffset(selection.Start), doc.Editor.ByteOffset(selection.End)
	ordinal := 0
	for index, span := range state.incrementMatches {
		if span.Start == start && span.End == end {
			ordinal = index + 1
			break
		}
	}
	return fmt.Sprintf("%d/%d", ordinal, len(state.incrementMatches))
}
