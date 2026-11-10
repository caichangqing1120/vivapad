package main

import (
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/indicators"
)

type indicatorTracker struct {
	editor  *text.Editor
	cursor  uint64
	length  int
	scratch []text.Edit
}

func trackIndicators(editor *text.Editor) indicatorTracker {
	return indicatorTracker{editor: editor, cursor: editor.TrackEdits(), length: len(editor.String())}
}

func (tracker *indicatorTracker) sync(editor *text.Editor, layers [][]text.Span) bool {
	if tracker.editor != editor {
		return false
	}
	edits, next, complete := editor.EditsSince(tracker.cursor, tracker.scratch)
	if !complete {
		return false
	}
	length := tracker.length
	for _, edit := range edits {
		for index, spans := range layers {
			updated, valid := indicators.Replace(spans, length, edit.Start, edit.Removed, edit.Inserted)
			if !valid {
				return false
			}
			layers[index] = updated
		}
		length += edit.Inserted - edit.Removed
	}
	if length != len(editor.String()) {
		return false
	}
	tracker.cursor, tracker.length, tracker.scratch = next, length, edits
	return true
}

func (state *model) syncFindMarks(doc *document.Document) bool {
	if state.markedDocument != doc {
		return true
	}
	layers := [1][]text.Span{state.markedSpans}
	if !state.markedTracking.sync(doc.Editor, layers[:]) {
		state.clearMarks()
		state.notice = "Marks cleared: edit history unavailable"
		return false
	}
	state.markedSpans, state.markedValue = layers[0], doc.Editor.String()
	return true
}
