package main

import (
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func (state *model) moveLinesWithBookmarks(doc *document.Document, up bool) {
	before := doc.Editor.String()
	selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
	start := strings.LastIndexByte(before[:doc.Editor.ByteOffset(selection.Start)], '\n') + 1
	end := doc.Editor.ByteOffset(selection.End)
	endLineStart := strings.LastIndexByte(before[:end], '\n') + 1
	appendEOL := false
	if end > endLineStart || start == end {
		if newline := strings.IndexByte(before[end:], '\n'); newline >= 0 {
			end += newline + 1
		} else {
			end = len(before)
		}
		appendEOL = end == len(before) && strings.LastIndexByte(before[:end], '\n')+1 == endLineStart
	}
	lastLineStart := strings.LastIndexByte(before, '\n') + 1
	endEmpty := strings.LastIndexByte(before[:end], '\n')+1 == len(before)
	if up && start == 0 || !up && end == len(before) && !endEmpty || start == end && !(up && endEmpty && end == len(before)) {
		return
	}
	line := strings.Count(before[:start], "\n")
	delta, shift := 1, 0
	if up {
		delta = -1
		previous := strings.LastIndexByte(before[:max(0, start-1)], '\n') + 1
		shift = -utf8.RuneCountInString(before[previous:start])
	} else {
		nextEnd := len(before)
		if newline := strings.IndexByte(before[end:], '\n'); newline >= 0 {
			nextEnd = end + newline
		}
		shift = utf8.RuneCountInString(before[end:nextEnd]) + 1
	}
	anchor := selection.Start
	if caret == selection.Start {
		anchor = selection.End
	}
	state.editWithBookmarkAnchor(doc, func() {
		err := doc.Editor.Transact(func(draft *text.Editor) error {
			if !up && end == lastLineStart {
				draft.ReplaceRange(text.Range{Start: draft.RuneIndex(end - 1), End: draft.RuneIndex(end)}, "")
				end--
			}
			selected := before[start:end]
			deleteStart := start
			if appendEOL {
				deleteStart--
			}
			draft.ReplaceRange(text.Range{Start: draft.RuneIndex(deleteStart), End: draft.RuneIndex(end)}, "")
			value := draft.String()
			if line+delta >= strings.Count(value, "\n")+1 {
				draft.ReplaceRange(text.Range{Start: draft.RuneCount(), End: draft.RuneCount()}, "\n")
				value = draft.String()
			}
			position := 0
			for remaining := line + delta; remaining > 0; remaining-- {
				newline := strings.IndexByte(value[position:], '\n')
				if newline < 0 {
					position = len(value)
					break
				}
				position += newline + 1
			}
			at := draft.RuneIndex(position)
			draft.ReplaceRange(text.Range{Start: at, End: at}, selected)
			if appendEOL {
				at = draft.RuneIndex(position + len(selected))
				draft.ReplaceRange(text.Range{Start: at, End: at}, "\n")
			}
			draft.MoveTo(anchor+shift, false)
			draft.MoveTo(caret+shift, true)
			return nil
		})
		if err != nil {
			state.notice = err.Error()
		}
	})
}
