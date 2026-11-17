package searchfiles

import (
	"os"
	"path/filepath"
	"testing"

	"vivapad/internal/document"
)

func TestOpenEqualTargetsInChangedDocumentRetainSourceSavePointHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "equal.txt")
	if err := os.WriteFile(path, []byte("cat keep dog"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewOpenReplace([]*document.Document{doc}, `(cat|dog)`, "cat", document.SearchOptions{Regex: true})
	if err != nil || plan.Replacements != 2 || len(plan.Changes) != 1 || len(plan.Changes[0].Edits) != 2 {
		t.Fatalf("changed document lost its equal target: %+v, %v", plan, err)
	}
	if err := plan.Apply([]*document.Document{doc}); err != nil || doc.Editor.String() != "cat keep cat" || !doc.Dirty() || !doc.Editor.CanUndo() {
		t.Fatalf("open equal targets lost real history: dirty=%v undo=%v, %v", doc.Dirty(), doc.Editor.CanUndo(), err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "cat keep dog" || doc.Dirty() ||
		!doc.Editor.Redo() || doc.Editor.String() != "cat keep cat" || !doc.Dirty() {
		t.Fatal("open equal targets did not cross the saved boundary")
	}
	if saved, err := os.ReadFile(path); err != nil || string(saved) != "cat keep dog" {
		t.Fatal("open equal-target apply wrote disk bytes")
	}
}
