package main

import (
	"sort"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func (state *model) navigateMarks(previous bool) {
	doc := state.space.Current()
	if !state.syncFindMarks(doc) {
		return
	}
	if state.markedDocument != doc || len(state.markedSpans) == 0 {
		state.notice = "No marks in this document"
		return
	}
	navigateMarkSpans(doc, state.markedSpans, previous)
}

func navigateMarkSpans(doc *document.Document, spans []text.Span, previous bool) {
	if len(spans) == 0 {
		return
	}
	position := doc.Editor.ByteOffset(doc.Editor.Caret())
	index := sort.Search(len(spans), func(index int) bool {
		return spans[index].End > position
	})
	if previous {
		index--
		if index < 0 {
			index = len(spans) - 1
		}
	} else {
		if index < len(spans) && spans[index].Start <= position {
			index++
		}
		if index == len(spans) {
			index = 0
		}
	}
	span := spans[index]
	start, end := doc.Editor.RuneIndex(span.Start), doc.Editor.RuneIndex(span.End)
	if previous {
		doc.Editor.SetSelection(text.Range{Start: end, End: start})
	} else {
		doc.Editor.SetSelection(text.Range{Start: start, End: end})
	}
}
