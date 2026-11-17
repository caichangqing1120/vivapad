package document

// H03: document lifecycle binding for the change-history consumer.
//
// A Document owns exactly one change-history consumer, bound to its final editor
// at creation — after Open has built the editor with the decoded text and
// before the first undoable edit. Any editor replacement (Open, reload,
// directory replace, draft transaction) creates a new Document with a new
// consumer; a replaced or retired editor is never treated as the source.
//
// ChangeHistory is the single consumption path: it synchronises the bound
// producer and returns the consumer. A construction failure, a replaced editor
// or a producer failure (including the 256-frame ring gap) is a persistent
// explicit unavailable error, never a silent "no changes" answer.
//
// Contract: docs/specs/001-vivapad/h03-lifecycle-contract.md.

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/vivaui/viva/text"
	"vivapad/internal/changehistory"
)

// errNoChangeHistorySource reports a Document that was never bound to a
// change-history producer (a zero value, a transient edit wrapper or a draft).
var errNoChangeHistorySource = errors.New("document has no change-history source")

// NewDraft wraps editor in a Document for transient edits that are not the
// user-visible document: a transaction's private draft editor or another
// throwaway editor. It deliberately binds no history producer, so a draft can
// never capture or answer change-history facts for the editor it wraps. A
// producer cannot be attached later either (TrackHistory refuses an editor with
// live Undo history), which keeps the unavailable state explicit.
func NewDraft(name string, editor *text.Editor) *Document {
	doc := &Document{
		Name:          name,
		Editor:        editor,
		newline:       "\n",
		savedNewline:  "\n",
		historyErr:    errNoChangeHistorySource,
		historyEditor: editor,
	}
	if editor != nil && editor.TrackLFLabels(bytes.Repeat([]byte{'L'}, strings.Count(editor.String(), "\n")), 'L', mapLFLineEndings) {
		doc.eolEditor = editor
	}
	return doc
}

// bindChangeHistory enables the producer on the document's final editor. It must
// be called once the editor is in place and before the first undoable edit;
// late attachment is refused by the producer and recorded as unavailable.
// Binding is never used to replace a live source in place.
func (doc *Document) bindChangeHistory() {
	doc.history, doc.historyErr = nil, nil
	doc.historyEditor = doc.Editor
	doc.historyBound = true
	consumer, err := changehistory.NewConsumer(doc.Editor)
	if err != nil {
		doc.historyErr = fmt.Errorf("change history unavailable: %w", err)
		return
	}
	doc.history = consumer
}

// ChangeHistory returns the consumer bound when the document was created and
// synchronises it before returning. It is the single consumption path for the
// document's text and save history.
//
// An unavailable history is always an error: a construction failure, a
// replaced/retired editor, a lost producer checkpoint (for example an
// unconsumed 256-frame ring gap) and an invalidated consumer all return the
// explicit persistent cause. Callers must hide change markers and refuse
// navigation instead of drawing an empty mask.
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
		// A document's editor must only be replaced by creating a new Document.
		// Identity is checked because an equal-length replacement would otherwise
		// be accepted as a continuation of the old source.
		return nil, fmt.Errorf("%w: the editor was replaced without binding a new source", errNoChangeHistorySource)
	}
	if err := doc.history.Sync(); err != nil {
		return nil, err
	}
	return doc.history, nil
}

// ClearChangeHistory forgets undo/redo and the change-history editions without
// changing the text, encoding, EOL, caret/selection, clone ownership or the
// document's real unsaved boundary, then establishes a new history generation
// for the producer. It is the only place a live source is rebound: H03 refuses
// in-place editor replacement, but a proven reset legitimately starts a new
// bootstrap. A reset on an unavailable producer reports the explicit error.
func (doc *Document) ClearChangeHistory() error {
	if err := doc.ClearChangeHistoryPreflight(); err != nil {
		return err
	}
	doc.Editor.ClearChangeHistory()
	doc.bindChangeHistory()
	return doc.historyErr
}

// ClearChangeHistoryPreflight reports whether ClearChangeHistory can run without
// mutating anything. Command layers use it before their own preprocessing (for
// example cancelling a composition) so a refusal never has side effects.
func (doc *Document) ClearChangeHistoryPreflight() error {
	if doc == nil || doc.Editor == nil {
		return errNoChangeHistorySource
	}
	if !doc.historyBound || doc.historyEditor != doc.Editor {
		// A draft, a never-bound document or a replaced editor has no legitimate
		// source to reset; refusing before the reset keeps their state untouched.
		return errNoChangeHistorySource
	}
	if doc.Editor.CompositionActive() {
		// A live pre-edit owns rollback state that the reset would discard while
		// leaving the composition stranded; refuse without side effects.
		return fmt.Errorf("clear change history refused while a composition is active")
	}
	return nil
}
