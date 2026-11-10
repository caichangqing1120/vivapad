package document

import (
	"encoding/csv"
	"encoding/hex"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/indicators"
)

func TestCommentTargetsMatchOriginalScintillaForwardUndoRedo(t *testing.T) {
	file, err := os.Open("testdata/scintilla-comments.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 8
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	decode := func(value string) string {
		t.Helper()
		if value == "-" {
			return ""
		}
		decoded, err := hex.DecodeString(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(decoded)
	}
	parseSpans := func(value string) []text.Span {
		t.Helper()
		if value == "-" {
			return nil
		}
		var spans []text.Span
		for _, pair := range strings.Split(value, ",") {
			start, end, found := strings.Cut(pair, ":")
			first, firstErr := strconv.Atoi(start)
			last, lastErr := strconv.Atoi(end)
			if !found || firstErr != nil || lastErr != nil {
				t.Fatalf("invalid comment oracle span %q", pair)
			}
			spans = append(spans, text.Span{Start: first, End: last})
		}
		return spans
	}
	var doc *Document
	var spans []text.Span
	var cursor uint64
	length, checked := 0, 0
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		changed := false
		if fields[5] == "forward" {
			doc = New("oracle.go")
			doc.Editor.Insert(decode(fields[1]))
			doc.Editor.ClearHistory()
			start, startErr := strconv.Atoi(fields[2])
			end, endErr := strconv.Atoi(fields[3])
			if startErr != nil || endErr != nil {
				t.Fatal("invalid comment oracle selection")
			}
			doc.Editor.SetSelection(text.Range{Start: start, End: end})
			spans = parseSpans(fields[4])
			length, cursor = len(doc.Editor.String()), doc.Editor.TrackEdits()
			switch fields[0] {
			case "line-add":
				changed = doc.ChangeLineComments("//", LineCommentAdd)
			case "line-toggle":
				changed = doc.ChangeLineComments("//", LineCommentToggle)
			case "stream-line-add":
				changed = doc.ChangeStreamLineComments("/*", "*/", LineCommentAdd)
			case "block-add", "empty-block-add", "blank-block-add":
				changed = doc.AddBlockComment("/*", "*/")
			case "block-remove", "empty-block-remove":
				changed = doc.RemoveBlockComments("/*", "*/")
			default:
				t.Fatalf("unknown comment oracle scenario %q", fields[0])
			}
		} else if doc != nil && fields[5] == "undo" {
			changed = doc.Editor.Undo()
		} else if doc != nil && fields[5] == "redo" {
			changed = doc.Editor.Redo()
		} else {
			t.Fatalf("invalid oracle phase %q", fields[5])
		}
		if !changed {
			t.Fatalf("comment oracle action refused: %s %s", fields[0], fields[5])
		}
		edits, next, complete := doc.Editor.EditsSince(cursor, nil)
		if !complete {
			t.Fatal("comment oracle edit journal is incomplete")
		}
		for _, edit := range edits {
			updated, valid := indicators.Replace(spans, length, edit.Start, edit.Removed, edit.Inserted)
			if !valid {
				t.Fatalf("invalid comment primitive: %+v", edit)
			}
			spans, length = updated, length+edit.Inserted-edit.Removed
		}
		cursor = next
		if doc.Editor.String() != decode(fields[6]) || !slices.Equal(spans, parseSpans(fields[7])) {
			t.Fatalf("comment oracle row %d (%s/%s initial=%s): text=%q spans=%+v want=%s", checked, fields[0], fields[5], fields[4], doc.Editor.String(), spans, fields[7])
		}
		checked++
	}
	if checked != 2181 {
		t.Fatalf("checked %d comment oracle rows, want 2181", checked)
	}
}
