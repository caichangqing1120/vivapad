package document

import (
	"testing"

	"github.com/vivaui/viva/text"
	"golang.org/x/text/language"
)

func TestRectangleLocaleSortUsesRowKeysAndRetainsDirection(t *testing.T) {
	before := "outside\nz 22 keep\na 1 keep\nm 3 keep\ntail"
	want := "outside\na 1 keep\nm 3 keep\nz 22 keep\ntail"
	for _, reverseVertical := range []bool{false, true} {
		for _, reverseHorizontal := range []bool{false, true} {
			doc := New("locale-rectangle.txt")
			doc.Editor.Insert(before)
			doc.Editor.ClearHistory()
			first, last := len("outside\n"), len("outside\nz 22 keep\na 1 keep\n")
			anchorColumn, caretColumn := 2, 4
			if reverseHorizontal {
				anchorColumn, caretColumn = caretColumn, anchorColumn
			}
			rect := Rectangle{Anchor: first + anchorColumn, Caret: last + caretColumn, TabWidth: 4}
			if reverseVertical {
				rect.Anchor, rect.Caret = rect.Caret, rect.Anchor
			}
			doc.Editor.MoveTo(rect.Caret, false)
			cursor := doc.Editor.TrackEdits()
			after, accepted, err := doc.SortRectangleLocale(rect, language.English, true)
			if err != nil || !accepted || doc.Editor.String() != want {
				t.Fatalf("Locale rectangle = %q accepted=%v err=%v", doc.Editor.String(), accepted, err)
			}
			firstAfter, lastAfter := len("outside\n"), len("outside\na 1 keep\nm 3 keep\n")
			expected := Rectangle{Anchor: firstAfter + anchorColumn, Caret: lastAfter + caretColumn, TabWidth: 4}
			if reverseVertical {
				expected.Anchor = lastAfter + caretColumn
				expected.Caret = firstAfter + anchorColumn
			}
			if after != expected || doc.Editor.Caret() != after.Caret {
				t.Fatalf("Locale direction = %+v caret=%d want %+v", after, doc.Editor.Caret(), expected)
			}
			edits, _, complete := doc.Editor.EditsSince(cursor, nil)
			if !complete || len(edits) != 1 || edits[0].Start != first || edits[0].Removed != len("z 22 keep\na 1 keep\nm 3 keep\n") {
				t.Fatalf("Locale did not replace only its full line block: %+v", edits)
			}
			if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() || !doc.Editor.Redo() || doc.Editor.String() != want || doc.Editor.Caret() != after.Caret {
				t.Fatal("Locale rectangle was not one atomic history unit")
			}
		}
	}
}

func TestRectangleLocaleSortVirtualSuffixAndTerminalLine(t *testing.T) {
	for _, test := range []struct {
		name, before, want string
		rect, after        Rectangle
	}{
		{"suffix", "z 20\na 2\nm 3", "a 2\nm 3\nz 20", Rectangle{Anchor: 2, Caret: 11, TabWidth: 4}, Rectangle{Anchor: 2, Caret: 10, TabWidth: 4}},
		{"short virtual", "z 20\na\nm 3", "a\nm 3\nz 20", Rectangle{Anchor: 2, Caret: 10, CaretVirtual: 1, TabWidth: 4}, Rectangle{Anchor: 1, AnchorVirtual: 1, Caret: 10, TabWidth: 4}},
		{"terminal empty", "z2\na1\n", "\na1\nz2", Rectangle{Anchor: 1, Caret: 6, CaretVirtual: 2, TabWidth: 4}, Rectangle{Anchor: 0, AnchorVirtual: 1, Caret: 6, TabWidth: 4}},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := New("locale-rectangle.txt")
			doc.Editor.Insert(test.before)
			doc.Editor.ClearHistory()
			doc.Editor.MoveTo(test.rect.Caret, false)
			after, accepted, err := doc.SortRectangleLocale(test.rect, language.English, true)
			if err != nil || !accepted || doc.Editor.String() != test.want || after != test.after {
				t.Fatalf("%s text=%q after=%+v accepted=%v err=%v, want %q %+v", test.name, doc.Editor.String(), after, accepted, err, test.want, test.after)
			}
		})
	}
}

func TestRectangleLocaleEqualResultHasRealHistoryAndSingleRowHasNone(t *testing.T) {
	doc := New("locale-equal.txt")
	before := "z 1\na 2"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	rect := Rectangle{Anchor: 2, Caret: 7, TabWidth: 4}
	doc.Editor.MoveTo(rect.Caret, false)
	cursor := doc.Editor.TrackEdits()
	after, accepted, err := doc.SortRectangleLocale(rect, language.English, true)
	edits, _, complete := doc.Editor.EditsSince(cursor, nil)
	if err != nil || !accepted || after != rect || doc.Editor.String() != before || !doc.Editor.CanUndo() || !complete || len(edits) != 1 {
		t.Fatalf("Locale equal replacement was skipped: rect=%+v accepted=%v err=%v edits=%+v", after, accepted, err, edits)
	}
	if !doc.Editor.Undo() || doc.Editor.CanUndo() || !doc.Editor.Redo() || doc.Editor.String() != before {
		t.Fatal("Locale equal rectangle history was not retained")
	}
	for _, single := range []Rectangle{{Anchor: 0, Caret: 0, TabWidth: 4}, {Anchor: 0, Caret: 1, TabWidth: 4}, {Anchor: 3, Caret: 3, AnchorVirtual: 1, CaretVirtual: 3, TabWidth: 4}} {
		doc.Editor.ClearHistory()
		result, accepted, err := doc.SortRectangleLocale(single, language.English, true)
		if err != nil || accepted || result != single || doc.Editor.String() != before || doc.Editor.CanUndo() {
			t.Fatalf("single-row Locale incorrectly sorted the document: %+v %v %v", result, accepted, err)
		}
	}
}

func TestRectangleLocaleRefusalIsAtomic(t *testing.T) {
	for _, composition := range []bool{false, true} {
		doc := New("locale-refused.txt")
		doc.Editor.Insert("z 2\na 1")
		doc.Editor.ClearHistory()
		doc.Editor.MoveTo(7, false)
		if composition {
			doc.Editor.SetMarkedText("", text.Range{})
		} else {
			doc.Editor.Restrict(func(rune) bool { return true }, 0)
		}
		before := doc.Editor.String()
		rect := Rectangle{Anchor: 2, Caret: doc.Editor.Caret(), TabWidth: 4}
		selection, cursor := doc.Editor.Selection(), doc.Editor.TrackEdits()
		after, accepted, err := doc.SortRectangleLocale(rect, language.English, true)
		edits, _, complete := doc.Editor.EditsSince(cursor, nil)
		if accepted || err == nil || after != rect || doc.Editor.String() != before || doc.Editor.Selection() != selection || len(edits) != 0 || !complete {
			t.Fatalf("refused Locale changed state: %+v %v %v %q", after, accepted, err, doc.Editor.String())
		}
	}
}
