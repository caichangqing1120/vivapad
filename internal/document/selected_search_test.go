package document

import (
	"testing"

	"github.com/vivaui/viva/text"
)

func TestSelectedSearchTargetResolvesCurrentWordWithoutSelecting(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("cat dog cat")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	if got, target := doc.SelectedSearchTarget(true); got != "cat" || target != (text.Range{Start: 0, End: 3}) || doc.Editor.Selection() != (text.Range{Start: 1, End: 1}) || doc.Editor.CanUndo() {
		t.Fatalf("expanded word = %q, target=%+v selection=%+v undo=%v", got, target, doc.Editor.Selection(), doc.Editor.CanUndo())
	}
	doc.Editor.SetSelection(text.Range{Start: 4, End: 7})
	if got, target := doc.SelectedSearchTarget(false); got != "dog" || target != (text.Range{Start: 4, End: 7}) || doc.Editor.Selection() != target {
		t.Fatalf("selected text = %q, selection=%+v", got, doc.Editor.Selection())
	}
	doc.Editor.MoveTo(4, false)
	if got, target := doc.SelectedSearchTarget(false); got != "" || target != (text.Range{Start: 4, End: 4}) || doc.Editor.Selection() != target {
		t.Fatalf("volatile empty selection = %q, selection=%+v", got, doc.Editor.Selection())
	}
}

func TestSelectedSearchTargetHandlesUnicodeBoundaryAndPunctuation(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("猫語 e\u0301clair. next")
	doc.Editor.MoveTo(0, false)
	if got, _ := doc.SelectedSearchTarget(true); got != "猫語" {
		t.Fatalf("Unicode word = %q", got)
	}
	doc.Editor.MoveTo(6, false)
	if got, _ := doc.SelectedSearchTarget(true); got != "e\u0301clair" {
		t.Fatalf("combining-mark word = %q", got)
	}
	doc.Editor.MoveTo(11, false)
	if got, _ := doc.SelectedSearchTarget(true); got != "" {
		t.Fatalf("punctuation unexpectedly selected %q", got)
	}
}
