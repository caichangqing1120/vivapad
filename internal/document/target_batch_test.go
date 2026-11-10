package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestPreparedTargetBatchCopiesInputAndCommitsOnce(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("cat keep cat")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 8, End: 4})
	cursor := doc.Editor.TrackEdits()
	edits := []ReplacementEdit{{Start: 0, End: 3, Text: "X"}, {Start: 9, End: 12, Text: "X"}}
	batch, err := doc.PrepareTargetEdits(edits)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "cat keep cat" || doc.Editor.Caret() != 4 || doc.Editor.CanUndo() {
		t.Fatal("preparing target edits mutated source")
	}
	if journal, next, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(journal) != 0 || next != cursor {
		t.Fatal("preparation entered edit journal")
	}
	edits[0].Text, edits[1].Start = "wrong", 0
	if err := batch.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := batch.Apply(); err != nil || doc.Editor.String() != "X keep X" || doc.Editor.Selection() != (text.Range{Start: 2, End: 6}) || doc.Editor.Caret() != 2 {
		t.Fatalf("prepared target apply = %q %v caret %d, %v", doc.Editor.String(), doc.Editor.Selection(), doc.Editor.Caret(), err)
	}
	if err := batch.Apply(); err == nil {
		t.Fatal("prepared target batch was replayed")
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "cat keep cat" || doc.Editor.Caret() != 4 || doc.Editor.CanUndo() {
		t.Fatal("target capsule lost one-unit history")
	}
	if !doc.Editor.Redo() || doc.Editor.Caret() != 2 {
		t.Fatal("target capsule lost final selection on Redo")
	}
}

func TestPreparedTargetBatchRejectsChangedSourceWithoutExtraMutation(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		mutate func(*Document)
	}{
		{"text", func(doc *Document) { doc.Editor.Insert("!") }},
		{"selection", func(doc *Document) { doc.Editor.MoveTo(0, false) }},
		{"editor", func(doc *Document) { doc.Editor = text.NewEditor(doc.Editor.String()) }},
		{"encoding", func(doc *Document) { doc.SetEncoding(UTF16LE) }},
		{"EOL", func(doc *Document) { doc.SetLineEnding(CRLF) }},
		{"restriction", func(doc *Document) { doc.Editor.Restrict(func(rune) bool { return true }, 0) }},
		{"composition", func(doc *Document) { doc.Editor.SetMarkedText("", text.Range{}) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			doc := New("new 1")
			doc.Editor.Insert("cat\ncat")
			doc.Editor.ClearHistory()
			doc.Editor.MoveTo(1, false)
			batch, err := doc.PrepareTargetEdits([]ReplacementEdit{{Start: 0, End: 3, Text: "X"}})
			if err != nil {
				t.Fatal(err)
			}
			scenario.mutate(doc)
			before, selection, caret := doc.Editor.String(), doc.Editor.Selection(), doc.Editor.Caret()
			cursor := doc.Editor.TrackEdits()
			if err := batch.Validate(); err == nil {
				t.Fatal("changed source passed target validation")
			}
			if err := batch.Apply(); err == nil || doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret {
				t.Fatalf("changed source partially applied: %q %v, %v", doc.Editor.String(), doc.Editor.Selection(), err)
			}
			if journal, next, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(journal) != 0 || next != cursor {
				t.Fatal("refused capsule entered edit journal")
			}
		})
	}
}

func TestPreparedTargetBatchRetainsMixedEOLFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("cat\r\ncat\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := doc.PrepareTargetEdits([]ReplacementEdit{{Start: 0, End: 3, Text: "X"}})
	if err != nil {
		t.Fatal(err)
	}
	doc.SetLineEnding(CRLF)
	if err := batch.Apply(); err == nil || doc.Editor.String() != "cat\ncat\n" || doc.Editor.CanUndo() {
		t.Fatalf("same-mode mixed EOL conversion was not stale: %q, %v", doc.Editor.String(), err)
	}
}

func TestPreparedTargetBatchCanRetryAfterNonmutatingRefusal(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("cat")
	doc.Editor.ClearHistory()
	batch, err := doc.PrepareTargetEdits([]ReplacementEdit{{Start: 0, End: 3, Text: "dog"}})
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.Restrict(func(rune) bool { return true }, 0)
	if err := batch.Apply(); err == nil {
		t.Fatal("restricted capsule applied")
	}
	doc.Editor.Restrict(nil, 0)
	if err := batch.Apply(); err != nil || doc.Editor.String() != "dog" {
		t.Fatalf("refused capsule was consumed: %q, %v", doc.Editor.String(), err)
	}
	var absent *TargetBatch
	if err := absent.Validate(); err == nil {
		t.Fatal("nil capsule validated")
	}
}
