package document

import (
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRectangleSortMatchesPinnedOriginalSortersAndSelection(t *testing.T) {
	file, err := os.Open("testdata/scintilla-rectangle-sort.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 14
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
		decode := func(index int) string {
			t.Helper()
			decoded, err := hex.DecodeString(fields[index])
			if err != nil {
				t.Fatal(err)
			}
			return string(decoded)
		}
		number := func(index int) int {
			t.Helper()
			value, err := strconv.Atoi(fields[index])
			if err != nil {
				t.Fatal(err)
			}
			return value
		}
		doc := New("rectangle-oracle.txt")
		before, expected := decode(0), decode(7)
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		rect := Rectangle{Anchor: doc.Editor.RuneIndex(number(1)), Caret: doc.Editor.RuneIndex(number(2)), AnchorVirtual: number(3), CaretVirtual: number(4), TabWidth: 4}
		doc.Editor.MoveTo(rect.Caret, false)
		cursor := doc.Editor.TrackEdits()
		changed, err := doc.SortRectangle(rect, SortMode(number(5)), number(6) != 0, nil)
		if (err != nil) != (number(9) != 0) || err != nil && !strings.Contains(err.Error(), fmt.Sprintf("line %d", number(9))) {
			t.Fatalf("row %d mode=%s expected failed line %s: %v", checked, fields[5], fields[9], err)
		}
		if changed != (number(8) != 0) || doc.Editor.String() != expected || doc.Editor.ByteOffset(doc.Editor.Caret()) != number(10) {
			t.Fatalf("row %d mode=%s rect=%+v before=%q: got %q changed=%v caret=%d, want %q changed=%s caret=%s", checked, fields[5], rect, before, doc.Editor.String(), changed, doc.Editor.ByteOffset(doc.Editor.Caret()), expected, fields[8], fields[10])
		}
		edits, _, complete := doc.Editor.EditsSince(cursor, nil)
		if !complete || len(edits) != number(8) || doc.Editor.CanUndo() != changed {
			t.Fatalf("row %d primitive/history mismatch: %+v", checked, edits)
		}
		if changed {
			if number(11) != number(10) || number(12) != number(10) || number(13) != 0 {
				t.Fatalf("row %d source rectangle did not collapse as expected: %v", checked, fields[10:])
			}
			if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() || !doc.Editor.Redo() || doc.Editor.String() != expected || doc.Editor.ByteOffset(doc.Editor.Caret()) != number(10) {
				t.Fatalf("row %d atomic history mismatch", checked)
			}
		}
		checked++
	}
	if checked != 7784 {
		t.Fatalf("checked %d original rectangle sort cases, want 7784", checked)
	}
	t.Logf("checked %d original rectangle sort/selection cases", checked)
}
