package document

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/vivaui/viva/text"
	"vivapad/internal/changehistory"
)

var errNoChangeHistorySource = errors.New("document has no change-history source")

// NewDraft wraps transient editor work without claiming user-visible history.
func NewDraft(name string, editor *text.Editor) *Document {
	if editor != nil {
		if _, enabled := editor.LFLabelRevision(); !enabled {
			labels := bytes.Repeat([]byte{'L'}, strings.Count(editor.String(), "\n"))
			editor.TrackLFLabels(labels, 'L', mapLFLineEndings)
		}
	}
	return &Document{
		Name: name, Editor: editor, newline: "\n", savedNewline: "\n",
		eolEditor: editor, historyEditor: editor, historyErr: errNoChangeHistorySource,
	}
}

func (doc *Document) bindChangeHistory() {
	doc.history, doc.historyErr = nil, nil
	doc.historyEditor = doc.Editor
	consumer, err := changehistory.NewConsumer(doc.Editor)
	if err != nil {
		doc.historyErr = fmt.Errorf("change history unavailable: %w", err)
		return
	}
	doc.history = consumer
}

// ChangeHistory synchronizes the producer bound to this document's final editor.
func (doc *Document) ChangeHistory() (*changehistory.Consumer, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil document: %w", errNoChangeHistorySource)
	}
	if doc.historyErr != nil {
		return nil, doc.historyErr
	}
	if doc.history == nil || doc.historyEditor == nil {
		return nil, errNoChangeHistorySource
	}
	if doc.Editor != doc.historyEditor {
		return nil, fmt.Errorf("%w: the editor was replaced without binding a new source", errNoChangeHistorySource)
	}
	if err := doc.history.Sync(); err != nil {
		return nil, err
	}
	return doc.history, nil
}

// ClearChangeHistory resets editions and Undo/Redo without changing the save boundary.
func (doc *Document) ClearChangeHistory() error {
	if doc == nil {
		return fmt.Errorf("nil document: %w", errNoChangeHistorySource)
	}
	if doc.historyErr != nil {
		return doc.historyErr
	}
	if doc.history == nil || doc.historyEditor == nil || doc.Editor != doc.historyEditor {
		return errNoChangeHistorySource
	}
	if doc.Editor.CompositionActive() {
		return errors.New("cannot clear change history during composition")
	}
	doc.Editor.ClearChangeHistory()
	doc.bindChangeHistory()
	return doc.historyErr
}
