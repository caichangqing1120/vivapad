package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestJoinLinesTouchesOnlySelectedLogicalLinesAndUndoes(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("a\nb\nc\n")
	doc.Editor.ClearHistory()
	if doc.JoinLines() || doc.Editor.CanUndo() {
		t.Fatal("collapsed join changed a document")
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	if doc.JoinLines() || doc.Editor.CanUndo() {
		t.Fatal("single-line join changed a document")
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 4})
	if !doc.JoinLines() || doc.Editor.String() != "a b\nc\n" {
		t.Fatalf("selected join = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\nb\nc\n" || doc.Editor.CanUndo() {
		t.Fatalf("join undo = %q", doc.Editor.String())
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 5})
	if !doc.JoinLines() || doc.Editor.String() != "a b c\n" {
		t.Fatalf("all selected lines joined = %q", doc.Editor.String())
	}
}

func TestJoinLinesSkipsExistingSpaceAndRoundTripsCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	if err := os.WriteFile(path, []byte("a \r\n\r\nb\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 5})
	if !doc.JoinLines() || doc.Editor.String() != "a b\n" {
		t.Fatalf("space-aware join = %q", doc.Editor.String())
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a b\r\n" {
		t.Fatalf("saved CRLF join = %q, %v", data, err)
	}
}

func TestReverseLinesUsesSelectedBoundsAndWholeDocumentEOFLine(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("猫\n犬\n鳥\n")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 4})
	if !doc.ReverseLines() || doc.Editor.String() != "犬\n猫\n鳥\n" {
		t.Fatalf("selected reverse = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "猫\n犬\n鳥\n" || doc.Editor.CanUndo() {
		t.Fatalf("reverse undo = %q", doc.Editor.String())
	}
	doc.Editor.MoveTo(0, false)
	if !doc.ReverseLines() || doc.Editor.String() != "\n鳥\n犬\n猫" {
		t.Fatalf("full reverse = %q", doc.Editor.String())
	}
}

func TestBlankLineAboveBelowCaretAndEOF(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("a\nb")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(2, false)
	doc.InsertBlankLineAbove()
	if doc.Editor.String() != "a\n\nb" || doc.Editor.Caret() != 2 {
		t.Fatalf("above second line = %q caret=%d", doc.Editor.String(), doc.Editor.Caret())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\nb" || doc.Editor.CanUndo() {
		t.Fatal("blank-above did not undo in one unit")
	}
	doc.Editor.MoveTo(0, false)
	doc.InsertBlankLineBelow()
	if doc.Editor.String() != "a\n\nb" || doc.Editor.Caret() != 2 {
		t.Fatalf("below first line = %q caret=%d", doc.Editor.String(), doc.Editor.Caret())
	}
	doc.Editor.Undo()
	doc.Editor.MoveTo(3, false)
	doc.InsertBlankLineBelow()
	if doc.Editor.String() != "a\nb\n" || doc.Editor.Caret() != 4 {
		t.Fatalf("below last line = %q caret=%d", doc.Editor.String(), doc.Editor.Caret())
	}
	doc.Editor.Undo()
	doc.Editor.MoveTo(0, false)
	doc.InsertBlankLineAbove()
	if doc.Editor.String() != "\na\nb" || doc.Editor.Caret() != 0 {
		t.Fatalf("above first line = %q caret=%d", doc.Editor.String(), doc.Editor.Caret())
	}
}

func TestBlankLineCRLFSavePreservesEncoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	if err := os.WriteFile(path, []byte("a\r\nb"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(2, false)
	doc.InsertBlankLineAbove()
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\n\r\nb" {
		t.Fatalf("saved blank CRLF = %q, %v", data, err)
	}
}
