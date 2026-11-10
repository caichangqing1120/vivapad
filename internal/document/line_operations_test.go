package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestSortSelectedLogicalLinesDoesNotPullInNextLine(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("z\nb\na\noutside\n")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 4})
	if !doc.SortLines(true) || doc.Editor.String() != "b\nz\na\noutside\n" {
		t.Fatalf("selected sort = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "z\nb\na\noutside\n" {
		t.Fatalf("sort undo = %q", doc.Editor.String())
	}
}

func TestSortWholeDocumentUsesSourceEOFLineAndCRLFEncoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	if err := os.WriteFile(path, []byte("z\r\nb\r\na\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.SortLines(true) || doc.Editor.String() != "\na\nb\nz" || !doc.Dirty() {
		t.Fatalf("sorted CRLF document = %q dirty=%v", doc.Editor.String(), doc.Dirty())
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if bytes, err := os.ReadFile(path); err != nil || string(bytes) != "\r\na\r\nb\r\nz" {
		t.Fatalf("saved sorted CRLF = %q, %v", bytes, err)
	}
	if !doc.SortLines(false) || doc.Editor.String() != "z\nb\na\n" {
		t.Fatalf("descending sort = %q", doc.Editor.String())
	}
}

func TestSortUnicodeSelectionUsesRuneSafeLineBoundaries(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("尾\n猫\n中\n首")
	doc.Editor.SetSelection(text.Range{Start: 2, End: 6})
	if !doc.SortLines(true) || doc.Editor.String() != "尾\n中\n猫\n首" {
		t.Fatalf("Unicode selected sort = %q", doc.Editor.String())
	}
}

func TestRemoveDuplicateLinesIncludesWholeDocumentEOFEmptyLine(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("b\n\nα\nb\n")
	doc.Editor.ClearHistory()
	if removed := doc.RemoveDuplicateLines(); removed != 2 || doc.Editor.String() != "b\n\nα" {
		t.Fatalf("deduplicated lines = %q removed=%d", doc.Editor.String(), removed)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "b\n\nα\nb\n" {
		t.Fatalf("dedup undo = %q", doc.Editor.String())
	}
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 0})
	if removed := doc.RemoveDuplicateLines(); removed != 2 || doc.Editor.String() != "b\n\nα" {
		t.Fatalf("nonconsecutive duplicate = %q removed=%d", doc.Editor.String(), removed)
	}
	if removed := doc.RemoveDuplicateLines(); removed != 0 || doc.Editor.String() != "b\n\nα" {
		t.Fatalf("repeated dedup should be a no-op: %q removed=%d", doc.Editor.String(), removed)
	}
}

func TestLineOperationsNoOpOnEmptyAndAlreadySortedText(t *testing.T) {
	doc := New("new 1")
	if doc.SortLines(true) || doc.RemoveDuplicateLines() != 0 || doc.RemoveEmptyLines(true) != 0 || doc.RemoveConsecutiveDuplicateLines() != 0 || doc.Dirty() {
		t.Fatal("empty line operation changed the document")
	}
	doc.Editor.Insert("A\na\né")
	doc.Editor.ClearHistory()
	if doc.SortLines(true) || doc.Editor.CanUndo() {
		t.Fatalf("already sorted text produced undo history: %q", doc.Editor.String())
	}
	if !doc.SortLines(false) || doc.Editor.String() != "é\na\nA" {
		t.Fatalf("descending Unicode order = %q", doc.Editor.String())
	}
}

func TestCaseInsensitiveSortIsStableAndUndoable(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("a\nB\nA\nb\n")
	doc.Editor.ClearHistory()
	if !doc.SortLinesIgnoreCase(true) || doc.Editor.String() != "\na\nA\nB\nb" {
		t.Fatalf("case-insensitive ascending = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\nB\nA\nb\n" {
		t.Fatalf("case-insensitive undo = %q", doc.Editor.String())
	}
	doc.Editor.ClearHistory()
	if !doc.SortLinesIgnoreCase(false) || doc.Editor.String() != "B\nb\na\nA\n" {
		t.Fatalf("case-insensitive descending = %q", doc.Editor.String())
	}
}

func TestRemoveEmptyAndWhitespaceOnlyLinesIncludesTerminalCleanup(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("first\n\n \t\nsecond\n")
	doc.Editor.ClearHistory()
	if removed := doc.RemoveEmptyLines(false); removed != 2 || doc.Editor.String() != "first\n \t\nsecond" {
		t.Fatalf("remove exact empty = %q removed=%d", doc.Editor.String(), removed)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "first\n\n \t\nsecond\n" {
		t.Fatalf("remove empty undo = %q", doc.Editor.String())
	}
	doc.Editor.ClearHistory()
	if removed := doc.RemoveEmptyLines(true); removed != 3 || doc.Editor.String() != "first\nsecond" {
		t.Fatalf("remove blank = %q removed=%d", doc.Editor.String(), removed)
	}
	doc = New("new 1")
	doc.Editor.Insert(" \t\n\n")
	if removed := doc.RemoveEmptyLines(true); removed != 2 || doc.Editor.String() != "" {
		t.Fatalf("all blank lines = %q removed=%d", doc.Editor.String(), removed)
	}
}

func TestRemoveEmptyLinesOnlyTouchesSelectedLogicalLines(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("head\n\n  \nbody\n")
	doc.Editor.SetSelection(text.Range{Start: 5, End: 9})
	if removed := doc.RemoveEmptyLines(true); removed != 2 || doc.Editor.String() != "head\nbody\n" {
		t.Fatalf("selected blank removal = %q removed=%d", doc.Editor.String(), removed)
	}
}

func TestConsecutiveDuplicateRemovalKeepsLaterOccurrence(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("b\nb\nc\nb\n")
	doc.Editor.ClearHistory()
	if removed := doc.RemoveConsecutiveDuplicateLines(); removed != 1 || doc.Editor.String() != "b\nc\nb\n" {
		t.Fatalf("consecutive dedup = %q removed=%d", doc.Editor.String(), removed)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "b\nb\nc\nb\n" {
		t.Fatalf("consecutive dedup undo = %q", doc.Editor.String())
	}
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 0})
	if removed := doc.RemoveConsecutiveDuplicateLines(); removed != 1 {
		t.Fatalf("consecutive dedup repeat = %d", removed)
	}
	if removed := doc.RemoveConsecutiveDuplicateLines(); removed != 0 || doc.Editor.String() != "b\nc\nb\n" {
		t.Fatalf("consecutive dedup no-op = %q removed=%d", doc.Editor.String(), removed)
	}
}
