package document

import (
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
)

func (doc *Document) sourceOrderBlock() lineBlock {
	block := doc.selectedLineBlock()
	selection := doc.Editor.Selection()
	if selection.Start == selection.End && block.terminated {
		block.lines = append(block.lines, "")
		block.terminated = false
	}
	return block
}

func (doc *Document) lineContentRange() text.Range {
	editor := doc.Editor
	selection := editor.Selection()
	if selection.Start != selection.End {
		block := doc.selectedLineBlock()
		end := block.end
		if block.terminated {
			end--
		}
		return text.Range{Start: editor.RuneIndex(block.start), End: editor.RuneIndex(end)}
	}
	value := editor.String()
	caret := editor.ByteOffset(editor.Caret())
	start, end := strings.LastIndexByte(value[:caret], '\n')+1, len(value)
	if newline := strings.IndexByte(value[caret:], '\n'); newline >= 0 {
		end = caret + newline
	}
	return text.Range{Start: editor.RuneIndex(start), End: editor.RuneIndex(end)}
}

func (doc *Document) replaceSourceLineBlock(block lineBlock, value string, after *text.Range) bool {
	if value == block.before {
		return false
	}
	batch, err := doc.PrepareTargetEdits([]ReplacementEdit{{Start: block.start, End: block.end, Text: value}})
	if err != nil {
		return false
	}
	if after != nil {
		batch.after = *after
	}
	return batch.Apply() == nil
}

func (doc *Document) replaceSortedLineBlock(block lineBlock, lines []string) bool {
	changed, err := doc.replaceSortedLineBlockOutcome(block, lines)
	return err == nil && changed
}

// replaceSortedLineBlockOutcome is replaceSortedLineBlock with the refusal
// cause, so callers can tell "nothing to change" from "the edit was refused".
func (doc *Document) replaceSortedLineBlockOutcome(block lineBlock, lines []string) (bool, error) {
	value := block.joined(lines)
	var after *text.Range
	selection := doc.Editor.Selection()
	if selection.Start != selection.End {
		start := doc.Editor.RuneIndex(block.start)
		end := start + utf8.RuneCountInString(value)
		if block.terminated {
			end--
		}
		after = &text.Range{Start: start, End: end}
	}
	if value == block.before {
		if after != nil {
			if err := doc.validateLineSelection(*after); err != nil {
				return false, err
			}
			doc.Editor.SetSelection(*after)
		}
		return false, nil
	}
	batch, err := doc.PrepareTargetEdits([]ReplacementEdit{{Start: block.start, End: block.end, Text: value}})
	if err != nil {
		return false, err
	}
	if after != nil {
		batch.after = *after
	}
	if err := batch.Apply(); err != nil {
		return false, err
	}
	return true, nil
}

func (doc *Document) validateLineSelection(after text.Range) error {
	return doc.Editor.ValidateReplaceRanges([]text.Replacement{{Range: text.Range{}}}, text.ReplaceRangesOptions{AfterSelection: &after})
}

func (doc *Document) insertLineTarget(position, caret int) {
	batch, err := doc.PrepareTargetEdits([]ReplacementEdit{{Start: position, End: position, Text: "\n"}})
	if err != nil {
		return
	}
	batch.after = text.Range{Start: caret, End: caret}
	_ = batch.Apply()
}
