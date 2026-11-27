package main

import (
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

const selectedFindLimit = 1024

func (state *model) selectedFind(backward, volatile bool) {
	doc := state.space.Current()
	query, target := doc.SelectedSearchTarget(!volatile)
	if query == "" {
		return
	}
	if !volatile && target.End-target.Start > selectedFindLimit {
		state.notice = "Selection exceeds 1024 characters"
		return
	}
	options := document.SearchOptions{}
	if !volatile {
		state.query = text.NewEditor(query)
		options = state.searchOptions
		options.Regex = false
		if selection := doc.Editor.Selection(); selection.Start == selection.End {
			doc.Editor.SetSelection(target)
		}
	}
	found, err := doc.FindWith(query, options, backward)
	if err != nil {
		state.notice = err.Error()
	} else if found {
		state.notice = ""
	} else {
		state.notice = "Text not found"
	}
}
