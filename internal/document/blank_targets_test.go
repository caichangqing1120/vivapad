package document

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/vivaui/viva/text"
)

func TestBlankTargetsFollowSourceBoundariesAndOneUndo(t *testing.T) {
	for _, scenario := range []struct {
		name          string
		mode          BlankOperation
		before, after string
		want          []text.Edit
	}{
		{"trailing", BlankTrimTrailing, "  \u03b1 \t\n\tkeep  \n", "  \u03b1\n\tkeep\n", []text.Edit{{Start: 4, Removed: 2}, {Start: 10, Removed: 2}}},
		{"leading", BlankTrimLeading, "  \u03b1 \t\n\tkeep  \n", "\u03b1 \t\nkeep  \n", []text.Edit{{Start: 0, Removed: 2}, {Start: 5, Removed: 1}}},
		{"both", BlankTrimBoth, "  \u03b1 \t\n\tkeep  \n", "\u03b1\nkeep\n", []text.Edit{{Start: 0, Removed: 2}, {Start: 2, Removed: 2}, {Start: 3, Removed: 1}, {Start: 7, Removed: 2}}},
		{"join", BlankEOLToSpace, "a \n b\n\nc\n", "a  b c ", []text.Edit{{Start: 2, Removed: 1}, {Start: 4, Removed: 1, Inserted: 1}, {Start: 5, Removed: 1}, {Start: 6, Removed: 1, Inserted: 1}}},
		{"trim then join", BlankTrimAndJoin, "  a \n b \n", "a b ", []text.Edit{{Start: 0, Removed: 2}, {Start: 1, Removed: 1}, {Start: 2, Removed: 1}, {Start: 3, Removed: 1}, {Start: 1, Removed: 1, Inserted: 1}, {Start: 3, Removed: 1, Inserted: 1}}},
		{"tabs", BlankTabToSpace, "a\tb\nkeep\n\tc", "a   b\nkeep\n    c", []text.Edit{{Start: 0, Removed: 3, Inserted: 5}, {Start: 11, Removed: 2, Inserted: 5}}},
		{"spaces all", BlankSpaceToTabAll, "a   b\nkeep\n    c", "a\tb\nkeep\n\tc", []text.Edit{{Start: 0, Removed: 5, Inserted: 3}, {Start: 9, Removed: 5, Inserted: 2}}},
		{"spaces leading", BlankSpaceToTabLeading, "    a  b\nkeep\n    c", "\ta  b\nkeep\n\tc", []text.Edit{{Start: 0, Removed: 8, Inserted: 5}, {Start: 11, Removed: 5, Inserted: 2}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			doc := New("blank.txt")
			doc.Editor.SetCode(text.CodeOptions{TabWidth: 4, HardTabs: true})
			doc.Editor.Insert(scenario.before)
			doc.Editor.ClearHistory()
			doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 0})
			beforeSelection := doc.Editor.Selection()
			cursor := doc.Editor.TrackEdits()
			if !doc.ApplyBlankOperation(scenario.mode) || doc.Editor.String() != scenario.after {
				t.Fatalf("result = %q, want %q", doc.Editor.String(), scenario.after)
			}
			edits, _, complete := doc.Editor.EditsSince(cursor, nil)
			if !complete || !slices.Equal(edits, scenario.want) {
				t.Fatalf("primitive targets = %+v complete=%v, want %+v", edits, complete, scenario.want)
			}
			afterSelection, afterCaret := doc.Editor.Selection(), doc.Editor.Caret()
			if scenario.mode != BlankEOLToSpace && afterSelection.Start != afterSelection.End && afterCaret != afterSelection.End {
				t.Fatal("changed line operation did not use source forward expanded selection")
			}
			if !doc.Editor.Undo() || doc.Editor.String() != scenario.before || doc.Editor.Selection() != beforeSelection || doc.Editor.Caret() != 0 || doc.Editor.CanUndo() {
				t.Fatal("blank Undo did not restore the original directional selection in one unit")
			}
			if !doc.Editor.Redo() || doc.Editor.String() != scenario.after || doc.Editor.Selection() != afterSelection || doc.Editor.Caret() != afterCaret {
				t.Fatal("blank Redo lost its final selection")
			}
		})
	}
}

func TestBlankTargetsExpandChangedSelectionAndKeepNoOpDirection(t *testing.T) {
	doc := New("selected.txt")
	doc.Editor.Insert(" outside \n  keep \t\n  next \n")
	doc.Editor.ClearHistory()
	start := strings.Index(doc.Editor.String(), "keep")
	doc.Editor.SetSelection(text.Range{Start: start + 2, End: start + 1})
	if !doc.ApplyBlankOperation(BlankTrimBoth) || doc.Editor.String() != " outside \nkeep\n  next \n" {
		t.Fatal("selected trim changed the wrong lines")
	}
	if doc.Editor.Selection() != (text.Range{Start: 10, End: 14}) || doc.Editor.Caret() != 14 {
		t.Fatalf("expanded source selection = %+v caret=%d", doc.Editor.Selection(), doc.Editor.Caret())
	}
	doc.Editor.SetSelection(text.Range{Start: 13, End: 11})
	if doc.ApplyBlankOperation(BlankTrimBoth) || doc.Editor.Caret() != 11 || doc.Editor.Selection() != (text.Range{Start: 11, End: 13}) {
		t.Fatal("no-op trim changed selection direction")
	}
	doc = New("empty.txt")
	doc.Editor.Insert(" \t\n a \n")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	if !doc.ApplyBlankOperation(BlankTrimAndJoin) || doc.Editor.String() != "\n a \n" || doc.Editor.Selection() != (text.Range{}) {
		t.Fatal("combined command joined after trimming the selected content to an empty selection")
	}
}

func TestBlankTabWholeDocumentUsesSourceCurrentLineCaret(t *testing.T) {
	for _, scenario := range []struct {
		mode          BlankOperation
		before, after string
		caret, want   int
	}{
		{BlankTabToSpace, "\u03b1\tkeep\nend", "\u03b1   keep\nend", 2, 4},
		{BlankTabToSpace, "\tfirst\nplain", "    first\nplain", 9, 10},
		{BlankSpaceToTabAll, "    keep\nend", "\tkeep\nend", 0, 0},
		{BlankSpaceToTabAll, "    keep\nend", "\tkeep\nend", 2, 0},
		{BlankSpaceToTabAll, "    keep\nend", "\tkeep\nend", 3, 1},
		{BlankSpaceToTabLeading, "    keep\nend", "\tkeep\nend", 4, 1},
	} {
		doc := New("caret.txt")
		doc.Editor.SetCode(text.CodeOptions{TabWidth: 4, HardTabs: true})
		doc.Editor.Insert(scenario.before)
		doc.Editor.ClearHistory()
		doc.Editor.MoveTo(scenario.caret, false)
		if !doc.ApplyBlankOperation(scenario.mode) || doc.Editor.String() != scenario.after || doc.Editor.Caret() != scenario.want || doc.Editor.Selection().Start != doc.Editor.Selection().End {
			t.Fatalf("mode=%d caret=%d: text=%q caret=%d, want %q caret=%d", scenario.mode, scenario.caret, doc.Editor.String(), doc.Editor.Caret(), scenario.after, scenario.want)
		}
	}
}

func TestBlankTargetsRefuseCompositionAndRestrictionsAtomically(t *testing.T) {
	for _, setup := range []struct {
		name  string
		apply func(*text.Editor)
	}{
		{"composition", func(editor *text.Editor) { editor.SetMarkedText("ni", text.Range{Start: 2, End: 2}) }},
		{"restricted", func(editor *text.Editor) { editor.Restrict(unicode.IsLetter, 100) }},
	} {
		for mode := BlankTrimTrailing; mode <= BlankSpaceToTabLeading; mode++ {
			doc := New("refused.txt")
			doc.Editor.Insert(" \talpha \t\n    beta \t\n")
			doc.Editor.ClearHistory()
			doc.Editor.SetSelection(text.Range{Start: 2, End: 7})
			setup.apply(doc.Editor)
			before, selection, caret := doc.Editor.String(), doc.Editor.Selection(), doc.Editor.Caret()
			composition, composing := doc.Editor.Composition()
			canUndo, canRedo := doc.Editor.CanUndo(), doc.Editor.CanRedo()
			cursor := doc.Editor.TrackEdits()
			if doc.ApplyBlankOperation(mode) || doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Editor.CanUndo() != canUndo || doc.Editor.CanRedo() != canRedo {
				t.Fatalf("%s mode %d partially edited a refused source", setup.name, mode)
			}
			if got, active := doc.Editor.Composition(); got != composition || active != composing {
				t.Fatal("refused blank command changed composition")
			}
			if edits, _, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
				t.Fatal("refused blank command changed primitive history")
			}
		}
	}
}

func TestBlankNoOpPreservesSavedStateAndRedo(t *testing.T) {
	for mode := BlankTrimTrailing; mode <= BlankSpaceToTabLeading; mode++ {
		doc := New("saved.txt")
		doc.Editor.Insert("keep")
		if err := doc.Save(filepath.Join(t.TempDir(), "saved.txt")); err != nil {
			t.Fatal(err)
		}
		doc.Editor.Insert("!")
		doc.Editor.Undo()
		doc.Editor.SetSelection(text.Range{Start: 3, End: 1})
		if doc.ApplyBlankOperation(mode) || doc.Dirty() || !doc.Editor.CanRedo() || doc.Editor.Caret() != 1 {
			t.Fatalf("no-op mode %d changed the saved boundary, selection or Redo", mode)
		}
	}
}

func TestBlankTargetsKeepUnaffectedMixedEOLOnSaveAndUndo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	before := "  alpha \r\nkeep\r  beta \n"
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.ApplyBlankOperation(BlankTrimBoth) || doc.EOLSequence() != "WML" {
		t.Fatal("trim changed untouched mixed line endings")
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(path); err != nil || string(saved) != "alpha\r\nkeep\rbeta\n" {
		t.Fatalf("saved blank bytes = %q, %v", saved, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "  alpha \nkeep\n  beta \n" || doc.EOLSequence() != "WML" {
		t.Fatal("blank Undo lost mixed EOL or text")
	}
}

func BenchmarkBlankOperationTargets(b *testing.B) {
	before := strings.Repeat("  alpha\tkeep \t\n", 256)
	for _, scenario := range []struct {
		name string
		mode BlankOperation
	}{
		{"trim", BlankTrimBoth},
		{"trim-join", BlankTrimAndJoin},
		{"tabs", BlankTabToSpace},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(before)))
			for b.Loop() {
				doc := New("benchmark.txt")
				doc.Editor.SetCode(text.CodeOptions{TabWidth: 4, HardTabs: true})
				doc.Editor.Insert(before)
				doc.Editor.ClearHistory()
				doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
				if !doc.ApplyBlankOperation(scenario.mode) {
					b.Fatal("blank benchmark did not edit")
				}
			}
		})
	}
}
