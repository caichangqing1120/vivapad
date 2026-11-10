package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestSplitLinesAtBreaksCurrentLineAsOneUndoUnit(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("alpha beta gamma\nnext")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	if !doc.SplitLinesAt([]int{6, 11}) || doc.Editor.String() != "alpha \nbeta \ngamma\nnext" {
		t.Fatalf("current line split = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "alpha beta gamma\nnext" || doc.Editor.CanUndo() {
		t.Fatalf("split Undo = %q", doc.Editor.String())
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	if doc.SplitLinesAt([]int{6, 11}) || doc.Editor.CanUndo() {
		t.Fatal("breaks on a different line created a change")
	}
}

func TestSplitLinesAtSelectedLinesAndFinalNewline(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("aa bb cc\ndd ee ff\ntail\n")
	doc.Editor.ClearHistory()
	start := len([]rune("aa bb cc\n"))
	end := len([]rune("aa bb cc\ndd ee ff\n"))
	doc.Editor.SetSelection(text.Range{Start: start, End: end})
	secondStart := len("aa bb cc\n")
	if !doc.SplitLinesAt([]int{3, 6, secondStart + 3, secondStart + 6}) || doc.Editor.String() != "aa bb cc\ndd \nee \nff\ntail\n" {
		t.Fatalf("selected split = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "aa bb cc\ndd ee ff\ntail\n" || doc.Editor.CanUndo() {
		t.Fatal("selected split did not undo as one unit")
	}
}

func TestSplitLinesAtRejectsMidRuneAndPreservesCRLFOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wide.txt")
	if err := os.WriteFile(path, []byte("猫犬鳥語\r\nlast"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(0, false)
	if !doc.SplitLinesAt([]int{1, len("猫犬")}) || doc.Editor.String() != "猫犬\n鳥語\nlast" {
		t.Fatalf("Unicode split = %q", doc.Editor.String())
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "猫犬\r\n鳥語\r\nlast" {
		t.Fatalf("saved Unicode CRLF split = %q, %v", data, err)
	}
	if strings.Contains(doc.Editor.String(), "\uFFFD") {
		t.Fatal("split inserted a replacement rune")
	}
}
