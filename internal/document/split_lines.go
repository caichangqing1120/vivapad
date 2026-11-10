package document

import (
	"slices"
)

func (doc *Document) SplitLinesAt(offsets []int) bool {
	value := doc.Editor.String()
	selection := doc.lineContentRange()
	start, end := doc.Editor.ByteOffset(selection.Start), doc.Editor.ByteOffset(selection.End)
	breaks := append([]int(nil), offsets...)
	slices.Sort(breaks)
	var changes []ReplacementEdit
	previous := start
	for _, at := range breaks {
		if at <= previous || at >= end || at >= len(value) || value[at] == '\n' || value[at-1] == '\n' ||
			doc.Editor.ByteOffset(doc.Editor.RuneIndex(at)) != at {
			continue
		}
		changes = append(changes, ReplacementEdit{Start: at, End: at, Text: "\n"})
		previous = at
	}
	if len(changes) == 0 {
		if doc.validateLineSelection(selection) == nil {
			doc.Editor.SetSelection(selection)
		}
		return false
	}
	batch, err := doc.PrepareTargetEdits(changes)
	if err != nil {
		return false
	}
	batch.mapSelection(selection)
	return batch.Apply() == nil
}
