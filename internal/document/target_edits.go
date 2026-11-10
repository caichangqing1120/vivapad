package document

import "github.com/vivaui/viva/text"

func (doc *Document) applyTargetEdits(edits []ReplacementEdit) error {
	if len(edits) == 0 {
		return nil
	}
	batch, err := doc.PrepareTargetEdits(edits)
	if err != nil {
		return err
	}
	return batch.Apply()
}

func moveTargetSelection(selection text.Range, start, end, inserted int) text.Range {
	deletePosition := func(position int) int {
		if position > start {
			if position > end {
				return position - (end - start)
			}
			return start
		}
		return position
	}
	anchor, caret := deletePosition(selection.Start), deletePosition(selection.End)
	anchorLeading, caretLeading := anchor < caret, caret < anchor
	if anchor > start || anchor == start && anchorLeading {
		anchor += inserted
	}
	if caret > start || caret == start && caretLeading {
		caret += inserted
	}
	return text.Range{Start: anchor, End: caret}
}
