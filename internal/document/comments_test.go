package document

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
)

func TestSingleLineCommentAndUncommentOnlySelectedLogicalLines(t *testing.T) {
	doc := New("draft.go")
	doc.Editor.Insert("  α\n\n  // β\nlast\n")
	doc.Editor.ClearHistory()
	end := utf8.RuneCountInString("  α\n\n  // β\n")
	doc.Editor.SetSelection(text.Range{Start: end, End: 0})
	if !doc.ChangeLineComments("//", LineCommentAdd) || doc.Editor.String() != "  // α\n\n  // // β\nlast\n" {
		t.Fatalf("line comment = %q", doc.Editor.String())
	}
	if doc.Editor.Caret() != 0 {
		t.Fatalf("backward selection direction changed: caret=%d", doc.Editor.Caret())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "  α\n\n  // β\nlast\n" || doc.Editor.CanUndo() {
		t.Fatal("line comment should be one undo unit")
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: end})
	if !doc.ChangeLineComments("//", LineCommentRemove) || doc.Editor.String() != "  α\n\n  β\nlast\n" {
		t.Fatalf("line uncomment = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "  α\n\n  // β\nlast\n" {
		t.Fatal("line uncomment should undo")
	}
}

func TestSingleLineCommentsUseCaretLineAndPreserveCRLFOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "comments.go")
	if err := os.WriteFile(path, []byte("first\r\n  second\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(9, false)
	if !doc.ChangeLineComments("//", LineCommentAdd) || doc.Editor.String() != "first\n  // second\n" {
		t.Fatalf("caret-line comment = %q", doc.Editor.String())
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "first\r\n  // second\r\n" {
		t.Fatalf("saved comments = %q, %v", data, err)
	}
	doc.Editor.MoveTo(0, false)
	if doc.ChangeLineComments("//", LineCommentRemove) || doc.ChangeLineComments("", LineCommentAdd) {
		t.Fatal("uncommented line or absent prefix should not edit")
	}
}

func TestToggleLineCommentsActsIndependentlyOnMixedSelectedLines(t *testing.T) {
	doc := New("mixed.go")
	before := "  // α\n  β\n\n\t// γ\nlast"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	end := utf8.RuneCountInString("  // α\n  β\n\n\t// γ\n")
	doc.Editor.SetSelection(text.Range{Start: end, End: 0})
	if !doc.ChangeLineComments("//", LineCommentToggle) || doc.Editor.String() != "  α\n  // β\n\n\tγ\nlast" {
		t.Fatalf("mixed toggle = %q", doc.Editor.String())
	}
	if doc.Editor.Caret() != 0 {
		t.Fatalf("reverse selection lost: %+v caret=%d", doc.Editor.Selection(), doc.Editor.Caret())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("toggle undo = %q", doc.Editor.String())
	}
	doc.Editor.MoveTo(utf8.RuneCountInString("  // α\n  β\n\n"), false)
	if doc.ChangeLineComments("", LineCommentToggle) || doc.Editor.CanUndo() {
		t.Fatal("missing prefix should be an unchanged no-op")
	}
}

func TestToggleStreamOnlyCommentsWrapsEachLineIndependently(t *testing.T) {
	doc := New("mixed.css")
	before := "  /* α */\n\tβ\n  /*γ*/\n"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: utf8.RuneCountInString(before)})
	if !doc.ChangeStreamLineComments("/*", "*/", LineCommentToggle) || doc.Editor.String() != "  α\n\t/* β */\n  γ\n" {
		t.Fatalf("stream-only toggle = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("stream-only toggle should be one undo unit")
	}
	doc.Editor.MoveTo(0, false)
	if doc.ChangeStreamLineComments("", "*/", LineCommentToggle) || doc.Editor.CanUndo() {
		t.Fatal("missing stream delimiter should not edit")
	}
}

func TestStreamLineCommentKeepsSelectionBeforeClosingDelimiter(t *testing.T) {
	doc := New("edge.css")
	doc.Editor.Insert("a\nb")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	if !doc.ChangeStreamLineComments("/*", "*/", LineCommentAdd) || doc.Editor.String() != "/* a */\nb" {
		t.Fatalf("stream line add = %q", doc.Editor.String())
	}
	if got := doc.Editor.Selection(); got != (text.Range{Start: 3, End: 4}) {
		t.Fatalf("selection includes closing delimiter: %+v", got)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\nb" {
		t.Fatal("stream line add did not undo")
	}
	doc.Editor.MoveTo(1, false)
	if !doc.ChangeStreamLineComments("/*", "*/", LineCommentAdd) || doc.Editor.Caret() != 4 {
		t.Fatalf("line-end caret moved beyond closing delimiter: %d", doc.Editor.Caret())
	}
}

func TestBlockCommentWrapsSelectedTextAndCaretLine(t *testing.T) {
	doc := New("draft.go")
	doc.Editor.Insert("  αβ\n  next\n")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 4, End: 2})
	if !doc.AddBlockComment("/*", "*/") || doc.Editor.String() != "  /* αβ */\n  next\n" {
		t.Fatalf("selected block comment = %q", doc.Editor.String())
	}
	if doc.Editor.Caret() != 5 || doc.Editor.Selection() != (text.Range{Start: 5, End: 7}) {
		t.Fatalf("reverse selection = %+v caret=%d", doc.Editor.Selection(), doc.Editor.Caret())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "  αβ\n  next\n" || doc.Editor.CanUndo() {
		t.Fatal("block comment did not undo in one step")
	}
	doc.Editor.MoveTo(8, false)
	if !doc.AddBlockComment("/*", "*/") || doc.Editor.String() != "  αβ\n  /* next */\n" {
		t.Fatalf("caret-line block comment = %q", doc.Editor.String())
	}
	if doc.Editor.Selection() != (text.Range{Start: 10, End: 14}) {
		t.Fatalf("caret-line content selection = %+v", doc.Editor.Selection())
	}
}

func TestBlockUncommentRemovesEnclosingAndAllSelectedPairsAsOneEdit(t *testing.T) {
	doc := New("draft.go")
	before := "before /* α */ middle /* β */ end"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	if !doc.RemoveBlockComments("/*", "*/") || doc.Editor.String() != "before α middle β end" {
		t.Fatalf("selected pair removal = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("multiple pair removal did not undo together")
	}
	doc.Editor.MoveTo(11, false)
	if !doc.RemoveBlockComments("/*", "*/") || doc.Editor.String() != "before α middle /* β */ end" {
		t.Fatalf("enclosing pair removal = %q", doc.Editor.String())
	}
	doc.Editor.Undo()
	doc.Editor.SetSelection(text.Range{Start: 10, End: 11})
	if !doc.RemoveBlockComments("/*", "*/") || doc.Editor.String() != "before α middle /* β */ end" {
		t.Fatalf("selected inner pair removal = %q", doc.Editor.String())
	}
}

func TestBlockUncommentHandlesTightDelimitersAndRejectsUnmatched(t *testing.T) {
	doc := New("draft.css")
	doc.Editor.Insert("x /*alpha*/ y")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(5, false)
	if !doc.RemoveBlockComments("/*", "*/") || doc.Editor.String() != "x alpha y" {
		t.Fatalf("tight delimiter removal = %q", doc.Editor.String())
	}
	doc.Editor.Undo()
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: doc.Editor.RuneCount()}, "x /* open")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(6, false)
	if doc.RemoveBlockComments("/*", "*/") || doc.Editor.CanUndo() || doc.Editor.String() != "x /* open" {
		t.Fatal("unmatched delimiter should be an unchanged no-op")
	}
}

func TestBlockCommentPreservesCRLFDiskFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "block.go")
	if err := os.WriteFile(path, []byte("a\r\nb\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(2, false)
	if !doc.AddBlockComment("/*", "*/") {
		t.Fatal("block comment did not edit")
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\n/* b */\r\n" {
		t.Fatalf("saved CRLF block comment = %q, %v", data, err)
	}
	if !doc.RemoveBlockComments("/*", "*/") {
		t.Fatal("block uncomment after save did not edit")
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\nb\r\n" {
		t.Fatalf("saved CRLF block uncomment = %q, %v", data, err)
	}
}
