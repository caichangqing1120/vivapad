package printing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestPrintJobSnapshotsUnsavedBufferAndSelection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.txt")
	if err := os.WriteFile(path, []byte("a\nb"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(1, false)
	doc.Editor.Insert("猫")
	doc.Editor.SetSelection(text.Range{Start: 1, End: 3})
	before, selection, undo := doc.Editor.String(), doc.Editor.Selection(), doc.Editor.CanUndo()
	job, err := Prepare(doc, ".txt", true)
	if err != nil || job.Title != doc.Path || job.Text != before || job.Language != ".txt" || !job.Interactive ||
		job.Scope != ScopeSelection || job.Selection != (text.Span{Start: 1, End: 5}) {
		t.Fatalf("print dialog job = %+v, %v", job, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\nb" {
		t.Fatalf("print snapshot saved unsaved text: %q, %v", data, err)
	}
	if doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.CanUndo() != undo {
		t.Fatal("preparing the print job mutated the editor")
	}
	doc.Editor.Insert("more")
	if job.Text != before || doc.Editor.String() == before {
		t.Fatal("print job did not retain an immutable live snapshot")
	}
}

func TestPrintNowUsesWholeBufferAndSourceDefaults(t *testing.T) {
	doc := document.New("new 1")
	doc.Editor.Insert("one 猫\ntwo")
	doc.Editor.SetSelection(text.Range{Start: 4, End: 5})
	before, selection := doc.Editor.String(), doc.Editor.Selection()
	job, err := Prepare(doc, ".go", false)
	if err != nil || job.Title != "new 1" || job.Text != before || job.Interactive ||
		job.Scope != ScopeAll || job.Selection != (text.Span{}) || job.Language != ".go" {
		t.Fatalf("Print Now job = %+v, %v", job, err)
	}
	if !job.Settings.LineNumbers || job.Settings.ColorMode != ColorOnWhite || job.Settings.FormFeedPageBreak ||
		job.Settings.MarginMM != (Margins{}) || job.Settings.Header != (TextBand{}) || job.Settings.Footer != (TextBand{}) {
		t.Fatalf("source default print settings = %+v", job.Settings)
	}
	if doc.Editor.String() != before || doc.Editor.Selection() != selection || !doc.Dirty() {
		t.Fatal("Print Now mutated the unsaved document")
	}
	interactive, err := Prepare(doc, ".go", true)
	if err != nil || interactive.Scope != ScopeSelection || interactive.Selection.End <= interactive.Selection.Start {
		t.Fatalf("interactive selection = %+v, %v", interactive, err)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 0})
	interactive, err = Prepare(doc, ".go", true)
	if err != nil || interactive.Scope != ScopeAll || interactive.Selection != (text.Span{}) {
		t.Fatalf("empty-selection print = %+v, %v", interactive, err)
	}
	if _, err := Prepare(nil, "", true); err == nil {
		t.Fatal("nil document accepted as a print job")
	}
}

func TestPrintJobNormalizesBackwardUnicodeSelectionAndAllowsEmptySource(t *testing.T) {
	doc := document.New("new 2")
	doc.Editor.Insert("α🙂z")
	doc.Editor.SetSelection(text.Range{Start: 3, End: 1})
	job, err := Prepare(doc, "", true)
	if err != nil || job.Scope != ScopeSelection || job.Selection != (text.Span{Start: 2, End: 7}) || job.Text != "α🙂z" {
		t.Fatalf("backward Unicode selection = %+v, %v", job, err)
	}
	empty, err := Prepare(document.New("new 3"), "", true)
	if err != nil || empty.Scope != ScopeAll || empty.Selection != (text.Span{}) || empty.Text != "" {
		t.Fatalf("empty source = %+v, %v", empty, err)
	}
}
