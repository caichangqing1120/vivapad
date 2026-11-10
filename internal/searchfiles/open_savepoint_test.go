package searchfiles

import (
	"os"
	"path/filepath"
	"testing"

	"vivapad/internal/document"
)

func TestOpenEqualTargetsRetainSourceSavePointHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "equal.txt")
	if err := os.WriteFile(path, []byte("cat keep cat"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewOpenReplace([]*document.Document{doc}, "cat", "cat", document.SearchOptions{})
	if err != nil || plan.Replacements != 2 || len(plan.Changes) != 1 {
		t.Fatalf("equal targets disappeared from open preview: %+v, %v", plan, err)
	}
	if err := plan.Apply([]*document.Document{doc}); err != nil || !doc.Dirty() || !doc.Editor.CanUndo() {
		t.Fatalf("open equal targets lost real history: dirty=%v undo=%v, %v", doc.Dirty(), doc.Editor.CanUndo(), err)
	}
	if !doc.Editor.Undo() || doc.Dirty() || !doc.Editor.Redo() || !doc.Dirty() {
		t.Fatal("open equal targets did not cross the saved boundary")
	}
	if saved, err := os.ReadFile(path); err != nil || string(saved) != "cat keep cat" {
		t.Fatal("open equal-target apply wrote disk bytes")
	}
}
