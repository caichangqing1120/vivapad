package document

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestDocumentCapturesLineFactsBeforeFirstBookmark(t *testing.T) {
	for _, opened := range []bool{false, true} {
		name := "new"
		if opened {
			name = "open"
		}
		t.Run(name, func(t *testing.T) {
			doc := New("history.txt")
			if opened {
				path := filepath.Join(t.TempDir(), "history.txt")
				if err := os.WriteFile(path, []byte("aa\nbb"), 0600); err != nil {
					t.Fatal(err)
				}
				var err error
				doc, err = Open(path)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				doc.Editor.Insert("aa\nbb")
			}
			doc.Editor.ClearHistory()
			if err := doc.Editor.Transact(func(draft *text.Editor) error {
				draft.ReplaceRange(text.Range{Start: 2, End: 3}, "")
				draft.ReplaceRange(text.Range{Start: 1, End: 1}, "\n")
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			cursor := doc.Editor.TrackLineEdits()
			if !doc.Editor.Undo() || doc.Editor.String() != "aa\nbb" {
				t.Fatal("transaction did not undo")
			}
			edits, next, complete := doc.Editor.LineEditsSince(cursor, nil)
			want := []text.LineEdit{
				{Edit: text.Edit{Start: 1, Removed: 1}, Line: 0, RemovedLines: 1},
				{Edit: text.Edit{Start: 2, Inserted: 1}, Line: 0, InsertedLines: 1, InsertedLF: true},
			}
			if !complete || next != cursor+2 || !slices.Equal(edits, want) {
				t.Fatalf("early history line facts = %v, cursor %d, complete %v; want %v", edits, next, complete, want)
			}
		})
	}
}
