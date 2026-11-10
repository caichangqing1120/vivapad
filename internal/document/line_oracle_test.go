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

func TestLineTargetsMatchPinnedSortersAndDecoration(t *testing.T) {
	file, err := os.Open("testdata/scintilla-line-targets.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 11
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
				t.Fatalf("invalid line oracle span %q", pair)
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
		switch fields[7] {
		case "forward":
			doc = New("line-oracle.txt")
			doc.Editor.Insert(decode(fields[2]))
			doc.Editor.ClearHistory()
			start, startErr := strconv.Atoi(fields[3])
			end, endErr := strconv.Atoi(fields[4])
			if startErr != nil || endErr != nil {
				t.Fatal("invalid line oracle selection")
			}
			doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneIndex(start), End: doc.Editor.RuneIndex(end)})
			spans = parseSpans(fields[6])
			length, cursor = len(doc.Editor.String()), doc.Editor.TrackEdits()
			var offsets []int
			if fields[5] != "-" {
				for _, raw := range strings.Split(fields[5], ",") {
					position, err := strconv.Atoi(raw)
					if err != nil {
						t.Fatal(err)
					}
					offsets = append(offsets, position)
				}
			}
			changed = applySourceLineAction(doc, fields[1], offsets)
		case "undo":
			changed = doc != nil && doc.Editor.Undo()
			if doc.Editor.CanUndo() {
				t.Fatal("one source line command left a partial Undo unit")
			}
		case "redo":
			changed = doc != nil && doc.Editor.Redo()
		default:
			t.Fatalf("unknown line oracle phase %q", fields[7])
		}
		if changed != (fields[10] == "1") {
			t.Fatalf("line oracle %s/%s changed=%v want %s", fields[0], fields[7], changed, fields[10])
		}
		edits, next, complete := doc.Editor.EditsSince(cursor, nil)
		if !complete {
			t.Fatal("line oracle journal is incomplete")
		}
		for _, edit := range edits {
			updated, valid := indicators.Replace(spans, length, edit.Start, edit.Removed, edit.Inserted)
			if !valid {
				t.Fatalf("invalid source line primitive %+v", edit)
			}
			spans, length = updated, length+edit.Inserted-edit.Removed
		}
		cursor = next
		if doc.Editor.String() != decode(fields[8]) || !slices.Equal(spans, parseSpans(fields[9])) {
			t.Fatalf("line oracle %d %s/%s initial=%s: text=%q spans=%+v, want %q spans=%s", checked, fields[0], fields[7], fields[6], doc.Editor.String(), spans, decode(fields[8]), fields[9])
		}
		checked++
	}
	if checked != 5214 {
		t.Fatalf("checked %d original line states, want 5214", checked)
	}
	t.Logf("checked %d original sorter/Decoration line states", checked)
}
