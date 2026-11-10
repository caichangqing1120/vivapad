package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestBlankTrimModesOnlyRemoveASCIIWhitespaceAndUndoOnce(t *testing.T) {
	input := " \t猫 \t\n\tb  \n\u00a0c\u00a0\n"
	for _, test := range []struct {
		mode BlankOperation
		want string
	}{
		{BlankTrimTrailing, " \t猫\n\tb\n\u00a0c\u00a0\n"},
		{BlankTrimLeading, "猫 \t\nb  \n\u00a0c\u00a0\n"},
		{BlankTrimBoth, "猫\nb\n\u00a0c\u00a0\n"},
	} {
		doc := New("new 1")
		doc.Editor.Insert(input)
		doc.Editor.ClearHistory()
		if !doc.ApplyBlankOperation(test.mode) || doc.Editor.String() != test.want {
			t.Fatalf("mode %d = %q, want %q", test.mode, doc.Editor.String(), test.want)
		}
		if !doc.Editor.Undo() || doc.Editor.String() != input || doc.Editor.CanUndo() {
			t.Fatalf("mode %d was not one undo unit: %q", test.mode, doc.Editor.String())
		}
	}
	doc := New("new 1")
	doc.Editor.Insert("猫\n")
	doc.Editor.ClearHistory()
	if doc.ApplyBlankOperation(BlankTrimBoth) || doc.Editor.CanUndo() {
		t.Fatal("no-op trim created an edit")
	}
}

func TestBlankTrimExpandsPartialSelectionButExcludesNextLineAtBoundary(t *testing.T) {
	value := " outside \n  selected \t\n  next  \n"
	doc := New("new 1")
	doc.Editor.Insert(value)
	start := strings.Index(value, "selected") + 2
	doc.Editor.SetSelection(text.Range{Start: start, End: start + 3})
	if !doc.ApplyBlankOperation(BlankTrimBoth) || doc.Editor.String() != " outside \nselected\n  next  \n" {
		t.Fatalf("partial-line trim = %q", doc.Editor.String())
	}
	doc.Editor.Undo()
	first := strings.Index(value, "outside")
	next := strings.Index(value, "  next")
	doc.Editor.SetSelection(text.Range{Start: first, End: next})
	if !doc.ApplyBlankOperation(BlankTrimBoth) || doc.Editor.String() != "outside\nselected\n  next  \n" {
		t.Fatalf("selection ending at next line included it: %q", doc.Editor.String())
	}
}

func TestBlankJoinLinesUsesScintillaSpaceBoundaryAndSelection(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("a \n b\n\nc\n")
	doc.Editor.ClearHistory()
	if !doc.ApplyBlankOperation(BlankEOLToSpace) || doc.Editor.String() != "a  b c " {
		t.Fatalf("join entire document = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a \n b\n\nc\n" {
		t.Fatal("join did not undo as one edit")
	}
	doc = New("new 2")
	doc.Editor.Insert("a\nb\nc")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	if !doc.ApplyBlankOperation(BlankEOLToSpace) || doc.Editor.String() != "a b\nc" {
		t.Fatalf("selected join touched the following newline: %q", doc.Editor.String())
	}
}

func TestBlankTrimAndJoinIsOneUndoUnit(t *testing.T) {
	doc := New("new 1")
	before := "  a  \n\t b \t\n"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	if !doc.ApplyBlankOperation(BlankTrimAndJoin) || doc.Editor.String() != "a b " {
		t.Fatalf("trim and join = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("combined operation undo = %q", doc.Editor.String())
	}
}

func TestBlankTrimAndJoinSelectionPreservesUnselectedFollowingEOL(t *testing.T) {
	doc := New("new 1")
	value := "  a  \n  b  \n  c  "
	doc.Editor.Insert(value)
	start := strings.Index(value, "a")
	end := strings.Index(value, "b") + 1
	doc.Editor.SetSelection(text.Range{Start: start, End: end})
	if !doc.ApplyBlankOperation(BlankTrimAndJoin) || doc.Editor.String() != "a b\n  c  " {
		t.Fatalf("selected trim/join crossed following EOL: %q", doc.Editor.String())
	}
}

func TestBlankTabAndSpaceConversionUsesTabStopsAndUnicodeRuneColumns(t *testing.T) {
	for _, test := range []struct {
		mode        BlankOperation
		input, want string
	}{
		{BlankTabToSpace, "a\t猫\tb\n\tc", "a   猫   b\n    c"},
		{BlankSpaceToTabAll, "a   b  c\n    d", "a\tb  c\n\td"},
		{BlankSpaceToTabLeading, "a   b  c\n    d", "a   b  c\n\td"},
		{BlankSpaceToTabAll, "abc d", "abc d"},
		{BlankSpaceToTabAll, "  \tX", "\tX"},
		{BlankSpaceToTabAll, "abc \tX", "abc\t\tX"},
		{BlankSpaceToTabLeading, "  \tX", "\tX"},
		{BlankSpaceToTabLeading, "abc \tX", "abc \tX"},
	} {
		doc := New("new 1")
		doc.Editor.SetCode(text.CodeOptions{TabWidth: 4, HardTabs: true})
		doc.Editor.Insert(test.input)
		changed := doc.ApplyBlankOperation(test.mode)
		if doc.Editor.String() != test.want || changed != (test.input != test.want) {
			t.Fatalf("mode %d input %q = %q, want %q changed=%v", test.mode, test.input, doc.Editor.String(), test.want, changed)
		}
	}
}

func TestBlankTabConversionOnlyChangesSelectedLogicalLines(t *testing.T) {
	doc := New("new 1")
	doc.Editor.SetCode(text.CodeOptions{TabWidth: 4, HardTabs: true})
	value := "a\tb\n\tc\n\td"
	doc.Editor.Insert(value)
	start := strings.Index(value, "c")
	doc.Editor.SetSelection(text.Range{Start: start, End: start + 1})
	if !doc.ApplyBlankOperation(BlankTabToSpace) || doc.Editor.String() != "a\tb\n    c\n\td" {
		t.Fatalf("selected TAB conversion crossed lines: %q", doc.Editor.String())
	}
}

func TestBlankTrimPreservesCRLFOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("  a \r\n b\t\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.ApplyBlankOperation(BlankTrimBoth) || !doc.Dirty() || doc.LineEnding() != CRLF {
		t.Fatalf("CRLF trim = %q, dirty=%v", doc.Editor.String(), doc.Dirty())
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\nb\r\n" {
		t.Fatalf("CRLF save = %q, %v", data, err)
	}
}
