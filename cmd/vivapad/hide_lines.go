package main

import (
	"slices"

	"github.com/vivaui/viva/components"
	"vivapad/internal/document"
)

type hiddenLineRange struct {
	Start int
	End   int
}

func (state *model) hiddenRanges(doc *document.Document) []hiddenLineRange {
	if state.hiddenValue[doc] != doc.Editor.String() {
		delete(state.hiddenLines, doc)
		delete(state.hiddenValue, doc)
	}
	return state.hiddenLines[doc]
}

func (state *model) hideSelectedLines(doc *document.Document) {
	state.hiddenRanges(doc)
	if doc.LineCount() < 3 {
		return
	}
	selection := doc.Editor.Selection()
	start, end := foldLineAt(doc, selection.Start), foldLineAt(doc, selection.End)
	start = max(1, start)
	end = min(doc.LineCount()-2, end)
	if end < start {
		return
	}
	ranges := append(state.hiddenLines[doc], hiddenLineRange{Start: start, End: end})
	slices.SortFunc(ranges, func(left, right hiddenLineRange) int { return left.Start - right.Start })
	merged := ranges[:0]
	for _, candidate := range ranges {
		if len(merged) != 0 && candidate.Start <= merged[len(merged)-1].End+1 {
			merged[len(merged)-1].End = max(merged[len(merged)-1].End, candidate.End)
		} else {
			merged = append(merged, candidate)
		}
	}
	state.hiddenLines[doc] = merged
	state.hiddenValue[doc] = doc.Editor.String()
	doc.GoToLine(start)
	state.invalidator.Invalidate()
}

func (state *model) showHiddenLines(doc *document.Document, header int) {
	ranges := state.hiddenRanges(doc)
	for index, hidden := range ranges {
		if hidden.Start-1 == header {
			state.hiddenLines[doc] = append(ranges[:index], ranges[index+1:]...)
			state.invalidator.Invalidate()
			return
		}
	}
}

func (state *model) visualFoldRanges(doc *document.Document) []components.CodeEditorFoldRange {
	ranges := state.foldRanges(doc)
	for _, hidden := range state.hiddenRanges(doc) {
		ranges = append(ranges, components.CodeEditorFoldRange{Header: hidden.Start - 1, LastLine: hidden.End})
	}
	slices.SortFunc(ranges, func(left, right components.CodeEditorFoldRange) int { return left.Header - right.Header })
	merged := ranges[:0]
	for _, candidate := range ranges {
		if len(merged) != 0 && candidate.Header <= merged[len(merged)-1].LastLine {
			merged[len(merged)-1].LastLine = max(merged[len(merged)-1].LastLine, candidate.LastLine)
		} else {
			merged = append(merged, candidate)
		}
	}
	return merged
}
