package document

import (
	"fmt"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
)

// TargetBatch is a single-use, caller-independent target transaction for one document.
type TargetBatch struct {
	doc          *Document
	editor       *text.Editor
	before       string
	encoding     Encoding
	ending       LineEnding
	eolSequence  string
	selection    text.Range
	caret        int
	replacements []text.Replacement
	after        text.Range
	applied      bool
}

// PrepareTargetEdits validates original UTF-8 byte targets and copies them without
// modifying source text, selection, history or the mutation journal.
func (doc *Document) PrepareTargetEdits(edits []ReplacementEdit) (*TargetBatch, error) {
	if doc == nil || doc.Editor == nil {
		return nil, fmt.Errorf("target document is unavailable")
	}
	value := doc.Editor.String()
	selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
	after := selection
	if caret == selection.Start {
		after = text.Range{Start: selection.End, End: selection.Start}
	}
	replacements := make([]text.Replacement, 0, len(edits))
	previousStart, previousEnd, shift := -1, -1, 0
	for index, edit := range edits {
		if edit.Start < 0 || edit.End < edit.Start || edit.End > len(value) ||
			index > 0 && (edit.Start <= previousStart || edit.Start < previousEnd) {
			return nil, fmt.Errorf("invalid target edit at index %d", index)
		}
		start, end := doc.Editor.RuneIndex(edit.Start), doc.Editor.RuneIndex(edit.End)
		if doc.Editor.ByteOffset(start) != edit.Start || doc.Editor.ByteOffset(end) != edit.End {
			return nil, fmt.Errorf("target edit at index %d splits a UTF-8 character", index)
		}
		inserted := utf8.RuneCountInString(edit.Text)
		after = moveTargetSelection(after, start+shift, end+shift, inserted)
		shift += inserted - (end - start)
		replacements = append(replacements, text.Replacement{Range: text.Range{Start: start, End: end}, Text: edit.Text})
		previousStart, previousEnd = edit.Start, edit.End
	}
	batch := &TargetBatch{
		doc: doc, editor: doc.Editor, before: value, encoding: doc.Encoding(),
		ending: doc.LineEnding(), eolSequence: doc.EOLSequence(), selection: selection, caret: caret,
		replacements: replacements, after: after,
	}
	if err := batch.Validate(); err != nil {
		return nil, err
	}
	return batch, nil
}

// Validate refuses stale or already-applied batches without changing the source.
func (batch *TargetBatch) Validate() error {
	if batch == nil || batch.applied || batch.doc == nil || batch.editor == nil {
		return fmt.Errorf("target batch is unavailable or already applied")
	}
	doc := batch.doc
	if doc.Editor != batch.editor || doc.Editor.String() != batch.before ||
		doc.Encoding() != batch.encoding || doc.LineEnding() != batch.ending || doc.EOLSequence() != batch.eolSequence ||
		doc.Editor.Selection() != batch.selection || doc.Editor.Caret() != batch.caret {
		return fmt.Errorf("%s changed after target preparation", doc.Name)
	}
	return batch.editor.ValidateReplaceRanges(batch.replacements, batch.options())
}

// Apply commits one validated batch. Coordinators must validate all independent
// sources first and keep the editor's single-threaded contract between commits.
func (batch *TargetBatch) Apply() error {
	if err := batch.Validate(); err != nil {
		return err
	}
	if err := batch.editor.ReplaceRangesWith(batch.replacements, batch.options()); err != nil {
		return err
	}
	batch.applied = true
	return nil
}

func (batch *TargetBatch) options() text.ReplaceRangesOptions {
	return text.ReplaceRangesOptions{Forward: true, AfterSelection: &batch.after, ApplyUnchanged: true}
}

func (batch *TargetBatch) mapSelection(selection text.Range) {
	shift := 0
	for _, replacement := range batch.replacements {
		inserted := utf8.RuneCountInString(replacement.Text)
		selection = moveTargetSelection(selection, replacement.Range.Start+shift, replacement.Range.End+shift, inserted)
		shift += inserted - (replacement.Range.End - replacement.Range.Start)
	}
	batch.after = selection
}
