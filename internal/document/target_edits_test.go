package document

import (
	"encoding/csv"
	"io"
	"os"
	"slices"
	"strconv"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestTargetSelectionMatchesPinnedScintillaOracle(t *testing.T) {
	file, err := os.Open("testdata/scintilla-selection.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 7
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var values [7]int
		for index, field := range fields {
			values[index], err = strconv.Atoi(field)
			if err != nil {
				t.Fatal(err)
			}
		}
		got := moveTargetSelection(text.Range{Start: values[0], End: values[1]}, values[2], values[2]+values[3], values[4])
		want := text.Range{Start: values[5], End: values[6]}
		if got != want {
			t.Fatalf("source selection case %d (%v) = %+v, want %+v", checked, values, got, want)
		}
		checked++
	}
	if checked != 1260 {
		t.Fatalf("checked %d source selection cases, want 1260", checked)
	}
}

func TestReplaceAllUsesForwardTargetsAndMappedSelectionHistory(t *testing.T) {
	doc := New("new 1")
	before := "cat keep cat tail"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 8, End: 4})
	cursor := doc.Editor.TrackEdits()
	count, edits, err := doc.ReplaceAllWithEdits("cat", "猫", SearchOptions{})
	if err != nil || count != 2 || len(edits) != 2 || doc.Editor.String() != "猫 keep 猫 tail" ||
		doc.Editor.Selection() != (text.Range{Start: 2, End: 6}) || doc.Editor.Caret() != 2 {
		t.Fatalf("target replace = %q %v caret %d, count %d, %v", doc.Editor.String(), doc.Editor.Selection(), doc.Editor.Caret(), count, err)
	}
	primitives, _, complete := doc.Editor.EditsSince(cursor, nil)
	if !complete || !slices.Equal(primitives, []text.Edit{{Start: 0, Removed: 3, Inserted: 3}, {Start: 9, Removed: 3, Inserted: 3}}) {
		t.Fatalf("Replace All rewrote non-target bytes: %+v", primitives)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.Caret() != 4 || doc.Editor.CanUndo() {
		t.Fatalf("target Undo = %q caret %d", doc.Editor.String(), doc.Editor.Caret())
	}
	if !doc.Editor.Redo() || doc.Editor.Selection() != (text.Range{Start: 2, End: 6}) || doc.Editor.Caret() != 2 {
		t.Fatalf("target Redo selection = %v caret %d", doc.Editor.Selection(), doc.Editor.Caret())
	}
}

func TestColumnEditorReplacesLineContentsButNotLineEndings(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("ab\ncd\n")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	cursor := doc.Editor.TrackEdits()
	changed, err := doc.EditColumn(ColumnEdit{Text: "X"}, 4)
	if err != nil || !changed || doc.Editor.String() != "Xab\nXcd\nX" || doc.Editor.Caret() != 0 {
		t.Fatalf("source column targets = %q caret %d, %v", doc.Editor.String(), doc.Editor.Caret(), err)
	}
	primitives, _, complete := doc.Editor.EditsSince(cursor, nil)
	want := []text.Edit{{Start: 0, Removed: 2, Inserted: 3}, {Start: 4, Removed: 2, Inserted: 3}, {Start: 8, Inserted: 1}}
	if !complete || !slices.Equal(primitives, want) {
		t.Fatalf("Column Editor included EOL or used point insertions: %+v", primitives)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd\n" || doc.Editor.CanUndo() || !doc.Editor.Redo() || doc.Editor.Caret() != 0 {
		t.Fatal("Column Editor target batch lost atomic history or mapped caret")
	}
}

func TestTargetBatchRejectsMalformedOrComposingInputAtomically(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("猫ab")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 3, End: 1})
	before, selection := doc.Editor.String(), doc.Editor.Selection()
	cursor := doc.Editor.TrackEdits()
	for _, edits := range [][]ReplacementEdit{
		{{Start: 1, End: 3, Text: "X"}},
		{{Start: 0, End: 3, Text: "狗"}, {Start: 4, End: 9, Text: "X"}},
		{{Start: 0, End: 4, Text: "X"}, {Start: 3, End: 5, Text: "Y"}},
	} {
		if err := doc.applyTargetEdits(edits); err == nil || doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.Caret() != 1 || doc.Editor.CanUndo() {
			t.Fatalf("invalid targets partially applied: %+v, text=%q selection=%v, %v", edits, doc.Editor.String(), doc.Editor.Selection(), err)
		}
	}
	if edits, _, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
		t.Fatalf("invalid target batch entered edit journal: %+v", edits)
	}
	doc.Editor.MoveTo(1, false)
	doc.Editor.SetMarkedText("ni", text.Range{Start: 2, End: 2})
	composing := doc.Editor.String()
	composition, active := doc.Editor.Composition()
	if _, _, err := doc.ReplaceAllWithEdits("a", "X", SearchOptions{}); err == nil || doc.Editor.String() != composing {
		t.Fatalf("target batch interrupted an active composition: %q, %v", doc.Editor.String(), err)
	}
	if got, stillActive := doc.Editor.Composition(); got != composition || stillActive != active {
		t.Fatal("refused target batch changed composition state")
	}
}
