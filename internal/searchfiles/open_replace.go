package searchfiles

import (
	"fmt"
	"slices"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

type OpenChange struct {
	Doc   *document.Document
	Name  string
	Count int
	Edits []document.ReplacementEdit

	before      string
	encoding    document.Encoding
	ending      document.LineEnding
	eolSequence string
	source      *document.Document
	editor      *text.Editor
	targets     []document.ReplacementEdit
}

type OpenReplacePlan struct {
	Changes      []OpenChange
	Replacements int
	applied      bool
}

func PreviewOpenReplace(docs []*document.Document, query, replacement string, options document.SearchOptions) (*OpenReplacePlan, error) {
	if len(docs) == 0 || query == "" {
		return nil, fmt.Errorf("enter text to replace in open documents")
	}
	plan := &OpenReplacePlan{}
	seen := make(map[*document.Document]bool, len(docs))
	editors := make(map[*text.Editor]*document.Document, len(docs))
	for _, doc := range docs {
		if doc == nil || doc.Editor == nil {
			return nil, fmt.Errorf("open document is unavailable")
		}
		if seen[doc] {
			continue
		}
		if owner := editors[doc.Editor]; owner != nil && owner != doc {
			return nil, fmt.Errorf("distinct open documents share one editor")
		}
		seen[doc], editors[doc.Editor] = true, doc
		before := doc.Editor.String()
		ending, sequence := doc.LineEnding(), doc.EOLSequence()
		// A replacement preview is a transient wrapper, not a document: it must
		// not bind a history producer to a blank editor it then replaces.
		candidate := document.NewDraft(doc.Name, text.NewEditor(before))
		candidate.Editor.SetSavePoint()
		candidate.SetEncoding(doc.Encoding())
		if err := candidate.RestoreLineEndings(ending, sequence); err != nil {
			return nil, fmt.Errorf("%s cannot preview replacement: %w", doc.Name, err)
		}
		count, edits, err := candidate.ReplaceAllWithEdits(query, replacement, options)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", doc.Name, err)
		}
		if count == 0 || candidate.Editor.String() == before {
			continue
		}
		if err := candidate.ValidateEncoding(); err != nil {
			return nil, fmt.Errorf("%w (%s cannot be saved after replacement)", err, doc.Name)
		}
		plan.Changes = append(plan.Changes, OpenChange{
			Doc: doc, Name: doc.Name, Count: count, Edits: append([]document.ReplacementEdit(nil), edits...),
			before: before, encoding: doc.Encoding(), ending: ending, eolSequence: sequence,
			source: doc, editor: doc.Editor, targets: edits,
		})
		plan.Replacements += count
	}
	return plan, nil
}

func (plan *OpenReplacePlan) Apply(open []*document.Document) error {
	if plan == nil || plan.applied {
		return fmt.Errorf("open-document replacement plan is unavailable or already applied")
	}
	current := make(map[*document.Document]bool, len(open))
	editors := make(map[*text.Editor]*document.Document, len(open))
	for _, doc := range open {
		if doc == nil || doc.Editor == nil {
			return fmt.Errorf("open document is unavailable; no documents were changed")
		}
		if owner := editors[doc.Editor]; owner != nil && owner != doc {
			return fmt.Errorf("distinct open documents share one editor; no documents were changed")
		}
		current[doc] = true
		editors[doc.Editor] = doc
	}
	batches := make([]*document.TargetBatch, 0, len(plan.Changes))
	seen := make(map[*document.Document]bool, len(plan.Changes))
	count := 0
	for _, change := range plan.Changes {
		if change.source == nil || change.Doc != change.source || seen[change.Doc] ||
			change.Count != len(change.targets) || !slices.Equal(change.Edits, change.targets) {
			return fmt.Errorf("replacement plan metadata changed; no documents were changed")
		}
		seen[change.Doc] = true
		if !current[change.Doc] || change.Doc.Editor != change.editor || change.Doc.Editor.String() != change.before ||
			change.Doc.Encoding() != change.encoding || change.Doc.LineEnding() != change.ending || change.Doc.EOLSequence() != change.eolSequence {
			return fmt.Errorf("%s changed after replacement preview; no documents were changed", change.Name)
		}
		batch, err := change.Doc.PrepareTargetEdits(change.targets)
		if err != nil {
			return fmt.Errorf("%s cannot apply target edits; no documents were changed: %w", change.Name, err)
		}
		batches = append(batches, batch)
		count += change.Count
	}
	if count != plan.Replacements {
		return fmt.Errorf("replacement count changed; no documents were changed")
	}
	for _, batch := range batches {
		if err := batch.Validate(); err != nil {
			return fmt.Errorf("source changed before target commit; no documents were changed: %w", err)
		}
	}
	for index, batch := range batches {
		if err := batch.Apply(); err != nil {
			return fmt.Errorf("target commit stopped after %d document(s): %w", index, err)
		}
	}
	plan.applied = true
	return nil
}
