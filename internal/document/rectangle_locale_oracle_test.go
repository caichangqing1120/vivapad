package document

import (
	"encoding/csv"
	"encoding/hex"
	"io"
	"os"
	"strconv"
	"testing"

	"golang.org/x/text/language"
)

func TestRectangleLocaleMatchesOriginalHandlerWithExplicitKeyAdapter(t *testing.T) {
	file, err := os.Open("testdata/scintilla-locale-rectangle.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 15
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
		number := func(index int) int {
			t.Helper()
			result, err := strconv.Atoi(fields[index])
			if err != nil {
				t.Fatal(err)
			}
			return result
		}
		decode := func(index int) string {
			t.Helper()
			result, err := hex.DecodeString(fields[index])
			if err != nil {
				t.Fatal(err)
			}
			return string(result)
		}
		doc := New("locale-handler.txt")
		before, expected := decode(0), decode(7)
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		rect := Rectangle{Anchor: doc.Editor.RuneIndex(number(1)), Caret: doc.Editor.RuneIndex(number(2)), AnchorVirtual: number(3), CaretVirtual: number(4), TabWidth: number(5)}
		doc.Editor.MoveTo(rect.Caret, false)
		cursor := doc.Editor.TrackEdits()
		after, accepted, err := doc.sortRectangleLocaleKeys(rect, newUnicodeLocaleKeys(language.English), number(6) != 0)
		if err != nil || accepted != (number(12) == 1) || doc.Editor.String() != expected || doc.Editor.ByteOffset(after.Anchor) != number(8) || doc.Editor.ByteOffset(after.Caret) != number(9) || after.AnchorVirtual != number(10) || after.CaretVirtual != number(11) || after.TabWidth != rect.TabWidth || doc.Editor.Caret() != after.Caret {
			t.Fatalf("row %d before=%q rect=%+v ascending=%s: text=%q after=%+v accepted=%v err=%v, want text=%q bytes=%s/%s virtual=%s/%s accepted=%s", checked, before, rect, fields[6], doc.Editor.String(), after, accepted, err, expected, fields[8], fields[9], fields[10], fields[11], fields[12])
		}
		edits, _, complete := doc.Editor.EditsSince(cursor, nil)
		if !complete || len(edits) != number(12) || doc.Editor.CanUndo() != accepted {
			t.Fatalf("row %d original replacement count/history mismatch: %+v", checked, edits)
		}
		if accepted {
			if edits[0].Start != number(13) || edits[0].Removed != number(14)-number(13) || edits[0].Inserted != edits[0].Removed {
				t.Fatalf("row %d original target scope mismatch: %+v", checked, edits)
			}
			if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() || !doc.Editor.Redo() || doc.Editor.String() != expected || doc.Editor.Caret() != after.Caret {
				t.Fatalf("row %d atomic Locale history mismatch", checked)
			}
		}
		checked++
	}
	if checked != 840 {
		t.Fatalf("checked %d original Locale handler cases, want 840", checked)
	}
	t.Logf("checked %d original Locale handler cases with an explicit host key/geometry adapter", checked)
}
