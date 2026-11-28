package document

import (
	"errors"
	"strings"

	"github.com/vivaui/viva/text"
)

type FileFormat struct {
	Encoding Encoding
	EOL      LineEnding
	Sequence string
}

type formatEdit struct {
	position      int
	before, after FileFormat
}

type formatHistory struct {
	editor     *text.Editor
	cursor     text.HistoryCursor
	revision   uint64
	position   int
	edits      []formatEdit
	applied    int
	redoCutoff int
	blocksRedo bool
}

func (doc *Document) FileFormat() FileFormat {
	return FileFormat{Encoding: doc.encoding, EOL: doc.LineEnding(), Sequence: doc.EOLSequence()}
}

func (doc *Document) ValidateFormatConversion(encoding Encoding, ending *LineEnding) error {
	if doc.large != nil {
		return ErrLargeFileReadOnly
	}
	if doc.Editor == nil || doc.Editor.CompositionActive() || !encoding.Valid() || ending != nil && *ending > CR {
		return errors.New("document format is not editable")
	}
	if _, err := doc.currentLFLabels(); err != nil {
		return err
	}
	var value string
	if ending == nil {
		physical, err := doc.physicalValue(doc.Editor.String())
		if err != nil {
			return err
		}
		value = physical
	} else {
		value = strings.ReplaceAll(doc.Editor.String(), "\n", newlineFor(*ending))
	}
	_, err := encodeText(value, encoding)
	return err
}

func (doc *Document) ConvertFormat(encoding Encoding, ending *LineEnding) error {
	if err := doc.ValidateFormatConversion(encoding, ending); err != nil {
		return err
	}
	before := doc.FileFormat()
	after := before
	after.Encoding = encoding
	if ending != nil {
		after.EOL, after.Sequence = *ending, ""
	}
	if before == after {
		return nil
	}
	history := doc.syncFormatHistory()
	if history == nil {
		doc.Editor.TrackHistory()
		_, cursor, _ := doc.Editor.HistorySince(text.HistoryCursor{})
		history = &formatHistory{editor: doc.Editor, cursor: cursor, revision: doc.Editor.TrackEdits()}
		doc.formatHistory = history
	}
	if err := doc.installFileFormat(after); err != nil {
		return err
	}
	if doc.Editor.CanRedo() {
		history.redoCutoff, history.blocksRedo = history.position, true
	}
	history.discardRedo()
	history.edits = append(history.edits, formatEdit{position: history.position, before: before, after: after})
	if len(history.edits) > 128 {
		copy(history.edits, history.edits[1:])
		history.edits = history.edits[:128]
	}
	history.applied = len(history.edits)
	doc.Editor.BreakUndoGroup()
	return nil
}

func (doc *Document) installFileFormat(format FileFormat) error {
	if format.EOL != doc.LineEnding() || format.Sequence != doc.EOLSequence() {
		uniform := doc.uniformEOL
		if err := doc.RestoreLineEndings(format.EOL, format.Sequence); err != nil {
			return err
		}
		doc.uniformEOL = uniform
	}
	doc.encoding = format.Encoding
	return nil
}

func (history *formatHistory) discardRedo() {
	clear(history.edits[history.applied:])
	history.edits = history.edits[:history.applied]
}

func (doc *Document) clearFormatUndo() {
	if history := doc.formatHistory; history != nil {
		history.edits, history.applied = nil, 0
	}
}

func (doc *Document) syncFormatHistory() *formatHistory {
	history := doc.formatHistory
	if history == nil {
		return nil
	}
	if history.editor != doc.Editor {
		doc.formatHistory = nil
		return nil
	}
	frames, cursor, ok := doc.Editor.HistorySince(history.cursor)
	revision := doc.Editor.TrackEdits()
	if !ok && (cursor != history.cursor || revision != history.revision) {
		*history = formatHistory{editor: doc.Editor}
	}
	history.cursor, history.revision = cursor, revision
	for _, frame := range frames {
		for _, step := range frame.Steps {
			switch step.Operation {
			case text.HistoryForward:
				history.discardRedo()
				history.blocksRedo = false
				history.position++
			case text.HistoryUndo:
				history.position--
			case text.HistoryRedo:
				history.position++
			}
		}
	}
	return history
}

func (history *formatHistory) canUndo() bool {
	return history != nil && history.applied > 0 && history.edits[history.applied-1].position == history.position
}

func (history *formatHistory) canRedo() bool {
	return history != nil && history.applied < len(history.edits) && history.edits[history.applied].position == history.position
}

func (history *formatHistory) allowsTextRedo() bool {
	return history == nil || !history.blocksRedo || history.position < history.redoCutoff
}

func (doc *Document) CanUndo() bool {
	return doc.syncFormatHistory().canUndo() || doc.Editor.CanUndo()
}

func (doc *Document) CanRedo() bool {
	history := doc.syncFormatHistory()
	return history.canRedo() || history.allowsTextRedo() && doc.Editor.CanRedo()
}

func (doc *Document) Undo() bool {
	history := doc.syncFormatHistory()
	if !history.canUndo() {
		return doc.Editor.Undo()
	}
	if doc.Editor.CompositionActive() || doc.installFileFormat(history.edits[history.applied-1].before) != nil {
		return false
	}
	history.applied--
	doc.Editor.BreakUndoGroup()
	return true
}

func (doc *Document) Redo() bool {
	history := doc.syncFormatHistory()
	if !history.canRedo() {
		return history.allowsTextRedo() && doc.Editor.Redo()
	}
	if doc.Editor.CompositionActive() || doc.installFileFormat(history.edits[history.applied].after) != nil {
		return false
	}
	history.applied++
	doc.Editor.BreakUndoGroup()
	return true
}
