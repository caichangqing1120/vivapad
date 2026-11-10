package document

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestRectangleSortUsesOriginalKeyColumns(t *testing.T) {
	for _, test := range []struct {
		name, before, after string
		first, last         int
		anchorCol, caretCol int
		anchorVS, caretVS   int
		mode                SortMode
	}{
		{"column", "outside\nz 2\na 1\nm 1\ntail", "outside\na 1\nm 1\nz 2\ntail", 1, 3, 2, 3, 0, 0, SortLexicographic},
		{"thin suffix", "z 2\na 1\nm 1", "a 1\nm 1\nz 2", 0, 2, 2, 2, 0, 0, SortLexicographic},
		{"zero column", "z\na\nm", "a\nm\nz", 0, 2, 0, 0, 0, 0, SortLexicographic},
		{"tabs not cells", "z\t2\na\t1", "a\t1\nz\t2", 0, 1, 2, 3, 0, 0, SortLexicographic},
		{"byte columns slice UTF16", "z猫a9Q\nm猫z1B", "m猫z1B\nz猫a9Q", 0, 1, 2, 5, 0, 0, SortLexicographic},
		{"virtual short keys", "zz9\na\nmm1", "a\nmm1\nzz9", 0, 2, 3, 2, 1, 0, SortLexicographic},
		{"integer", "z v10\na v2\nm v02", "m v02\na v2\nz v10", 0, 2, 2, 5, 0, 0, SortIntegerAware},
		{"decimal", "z 2.5\na -1.5\nm 2.5", "a -1.5\nz 2.5\nm 2.5", 0, 2, 2, 5, 0, 0, SortDecimalDot},
		{"comma", "z 2,5\na -1,5\nm 2,5", "a -1,5\nz 2,5\nm 2,5", 0, 2, 2, 5, 0, 0, SortDecimalComma},
		{"length", "z abcd\na a\nm abc", "a a\nm abc\nz abcd", 0, 2, 2, 2, 0, 0, SortLength},
		{"ignore case", "z B\na a\nm A", "a a\nm A\nz B", 0, 2, 2, 3, 0, 0, SortIgnoreCase},
		{"reverse scope", "outside\na\nb\nc\ntail", "outside\nc\nb\na\ntail", 1, 3, 0, 1, 0, 0, SortReverse},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := New("rectangle.txt")
			doc.Editor.Insert(test.before)
			doc.Editor.ClearHistory()
			lines := strings.Split(test.before, "\n")
			position := func(line, column int) int {
				result := column
				for _, content := range lines[:line] {
					result += len([]rune(content)) + 1
				}
				return result
			}
			rect := Rectangle{Anchor: position(test.first, test.anchorCol), Caret: position(test.last, test.caretCol), AnchorVirtual: test.anchorVS, CaretVirtual: test.caretVS, TabWidth: 4}
			doc.Editor.MoveTo(rect.Caret, false)
			cursor := doc.Editor.TrackEdits()
			changed, err := doc.SortRectangle(rect, test.mode, true, nil)
			if err != nil || !changed || doc.Editor.String() != test.after {
				t.Fatalf("sort = %q changed=%v err=%v, want %q", doc.Editor.String(), changed, err, test.after)
			}
			edits, _, complete := doc.Editor.EditsSince(cursor, nil)
			if !complete || len(edits) != 1 || edits[0].Removed == 0 || edits[0].Inserted != edits[0].Removed {
				t.Fatalf("not one original whole-block replacement: %+v", edits)
			}
			selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
			if !doc.Editor.Undo() || doc.Editor.String() != test.before || doc.Editor.CanUndo() || !doc.Editor.Redo() || doc.Editor.String() != test.after || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret {
				t.Fatal("rectangle sort is not one atomic Undo/Redo unit")
			}
		})
	}
}

func TestRectangleSortNoOpFailureAndRestriction(t *testing.T) {
	for _, test := range []struct {
		name, value string
		mode        SortMode
		composing   bool
		restricted  bool
		wantError   bool
	}{
		{"already sorted", "z 1\na 2", SortLexicographic, false, false, false},
		{"numeric failure", "z --\na 2", SortDecimalDot, false, false, true},
		{"composition", "z 2\na 1", SortLexicographic, true, false, true},
		{"restricted", "z 2\na 1", SortLexicographic, false, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := New("rectangle.txt")
			doc.Editor.Insert(test.value)
			doc.Editor.ClearHistory()
			doc.Editor.MoveTo(doc.Editor.RuneCount()-1, false)
			if test.composing {
				doc.Editor.SetMarkedText("", text.Range{})
			}
			if test.restricted {
				doc.Editor.Restrict(func(rune) bool { return true }, 100)
			}
			before := doc.Editor.String()
			rect := Rectangle{Anchor: 2, Caret: doc.Editor.Caret(), TabWidth: 4}
			selection, caret, cursor := doc.Editor.Selection(), doc.Editor.Caret(), doc.Editor.TrackEdits()
			changed, err := doc.SortRectangle(rect, test.mode, true, nil)
			edits, _, complete := doc.Editor.EditsSince(cursor, nil)
			if changed || (err != nil) != test.wantError || doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || len(edits) != 0 || !complete {
				t.Fatalf("refused/no-op sort changed state: changed=%v err=%v text=%q edits=%+v", changed, err, doc.Editor.String(), edits)
			}
		})
	}
}

func TestRectangleSortRandomUsesOnlySelectedLines(t *testing.T) {
	doc := New("rectangle.txt")
	doc.Editor.Insert("outside\nz\na\nm\ntail")
	doc.Editor.ClearHistory()
	_, err := doc.SortRectangle(Rectangle{Anchor: 8, Caret: 13, TabWidth: 4}, SortRandom, true, rand.New(rand.NewPCG(7, 1)))
	if err != nil || !strings.HasPrefix(doc.Editor.String(), "outside\n") || !strings.HasSuffix(doc.Editor.String(), "\ntail") {
		t.Fatalf("random rectangle scope = %q, %v", doc.Editor.String(), err)
	}
}

func TestRectangleSortSingleRowFollowsOriginalSelectionCountBranch(t *testing.T) {
	for _, test := range []struct {
		name            string
		anchor, caret   int
		anchorVS, endVS int
		want            string
	}{
		{"collapsed", 1, 1, 0, 0, "a\nb\nz"},
		{"virtual only", 1, 1, 1, 3, "a\nb\nz"},
		{"selected body", 0, 1, 0, 0, "z\nb\na"},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := New("one-row.txt")
			doc.Editor.Insert("z\nb\na")
			doc.Editor.ClearHistory()
			doc.Editor.MoveTo(test.caret, false)
			changed, err := doc.SortRectangle(Rectangle{Anchor: test.anchor, Caret: test.caret, AnchorVirtual: test.anchorVS, CaretVirtual: test.endVS, TabWidth: 4}, SortLexicographic, true, nil)
			if err != nil || doc.Editor.String() != test.want || changed != (test.want != "z\nb\na") {
				t.Fatalf("single-row source branch = %q, changed=%v err=%v", doc.Editor.String(), changed, err)
			}
		})
	}
}

func TestRectangleSortOneLineDocumentDoesNotParseNumbers(t *testing.T) {
	doc := New("one-line.txt")
	doc.Editor.Insert("--")
	doc.Editor.ClearHistory()
	changed, err := doc.SortRectangle(Rectangle{TabWidth: 4}, SortDecimalDot, true, nil)
	if changed || err != nil || doc.Editor.String() != "--" || doc.Editor.CanUndo() {
		t.Fatalf("single document line should not reach numeric parsing: %v, %v", changed, err)
	}
}
