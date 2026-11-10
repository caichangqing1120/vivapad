package document

import (
	"testing"

	"github.com/vivaui/viva/text"
)

func TestGoToLineClampsNothingAndKeepsUnicodeOffsets(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("甲\n乙\n丙")
	if !doc.GoToLine(2) || doc.Editor.Caret() != 2 {
		t.Fatalf("line 2 caret = %d", doc.Editor.Caret())
	}
	if doc.GoToLine(0) || doc.GoToLine(4) || doc.Editor.Caret() != 2 {
		t.Fatalf("invalid line changed caret to %d", doc.Editor.Caret())
	}
}

func TestDuplicateAndDeleteLogicalLineAreUndoable(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("甲\n乙\n丙")
	doc.Editor.MoveTo(2, false)
	doc.DuplicateLine()
	if doc.Editor.String() != "甲\n乙\n乙\n丙" || doc.Editor.Caret() != 4 {
		t.Fatalf("duplicate = %q caret=%d", doc.Editor.String(), doc.Editor.Caret())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "甲\n乙\n丙" {
		t.Fatalf("undo duplicate = %q", doc.Editor.String())
	}
	doc.Editor.MoveTo(2, false)
	doc.DeleteLine()
	if doc.Editor.String() != "甲\n丙" {
		t.Fatalf("delete = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "甲\n乙\n丙" {
		t.Fatalf("undo delete = %q", doc.Editor.String())
	}
}

func TestDuplicateLastLineWithoutFinalNewline(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("first\nlast")
	doc.DuplicateLine()
	if doc.Editor.String() != "first\nlast\nlast" {
		t.Fatalf("last duplicate = %q", doc.Editor.String())
	}
}

func TestIndentAndUnindentSelectedLinesAsOneEdit(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("甲\n乙\n丙")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 4})
	doc.IndentLines()
	if doc.Editor.String() != "\t甲\n\t乙\n丙" {
		t.Fatalf("indent = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "甲\n乙\n丙" {
		t.Fatalf("undo indent = %q", doc.Editor.String())
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 4})
	doc.IndentLines()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 6})
	doc.UnindentLines()
	if doc.Editor.String() != "甲\n乙\n丙" {
		t.Fatalf("unindent = %q", doc.Editor.String())
	}
}

func TestEmptyLastLineCanBeIndentedAndDeleted(t *testing.T) {
	doc := New("new 1")
	doc.IndentLines()
	if doc.Editor.String() != "\t" {
		t.Fatalf("indent empty line = %q", doc.Editor.String())
	}
	doc = New("new 1")
	doc.Editor.Insert("first\n")
	doc.DeleteLine()
	if doc.Editor.String() != "first" {
		t.Fatalf("delete trailing empty line = %q", doc.Editor.String())
	}
}

func TestIndentRespectsConfiguredCodeStyle(t *testing.T) {
	doc := New("source.go")
	doc.Editor.SetCode(text.CodeOptions{TabWidth: 4})
	doc.Editor.Insert("alpha")
	doc.Editor.MoveTo(0, false)
	doc.IndentLines()
	if got := doc.Editor.String(); got != "    alpha" {
		t.Fatalf("code indent style = %q", got)
	}
}
