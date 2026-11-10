package document

import (
	"testing"

	"github.com/vivaui/viva/text"
)

func TestSelectedSearchTextExpandsCurrentWordWithoutEditing(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("cat dog cat")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	if got := doc.SelectedSearchText(true); got != "cat" || doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) || doc.Editor.CanUndo() {
		t.Fatalf("expanded word = %q, selection=%+v undo=%v", got, doc.Editor.Selection(), doc.Editor.CanUndo())
	}
	doc.Editor.SetSelection(text.Range{Start: 4, End: 7})
	if got := doc.SelectedSearchText(false); got != "dog" || doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("selected text = %q, selection=%+v", got, doc.Editor.Selection())
	}
	doc.Editor.MoveTo(4, false)
	if got := doc.SelectedSearchText(false); got != "" || doc.Editor.Selection() != (text.Range{Start: 4, End: 4}) {
		t.Fatalf("volatile empty selection = %q, selection=%+v", got, doc.Editor.Selection())
	}
}

func TestSelectedSearchTextHandlesUnicodeBoundaryAndPunctuation(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("猫語 e\u0301clair. next")
	doc.Editor.MoveTo(0, false)
	if got := doc.SelectedSearchText(true); got != "猫語" {
		t.Fatalf("Unicode word = %q", got)
	}
	doc.Editor.MoveTo(6, false)
	if got := doc.SelectedSearchText(true); got != "e\u0301clair" {
		t.Fatalf("combining-mark word = %q", got)
	}
	doc.Editor.MoveTo(11, false)
	if got := doc.SelectedSearchText(true); got != "" {
		t.Fatalf("punctuation unexpectedly selected %q", got)
	}
}
