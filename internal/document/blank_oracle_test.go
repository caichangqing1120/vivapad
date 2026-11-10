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
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"vivapad/internal/indicators"
)

func TestBlankTargetsMatchOriginalDecorationForwardUndoRedo(t *testing.T) {
	file, err := os.Open("testdata/scintilla-blank-targets.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 9
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
				t.Fatalf("invalid blank oracle span %q", pair)
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
		switch fields[6] {
		case "forward":
			doc = New("oracle.txt")
			doc.Editor.SetCode(text.CodeOptions{TabWidth: 4, HardTabs: true})
			doc.Editor.Insert(decode(fields[2]))
			doc.Editor.ClearHistory()
			start, startErr := strconv.Atoi(fields[3])
			end, endErr := strconv.Atoi(fields[4])
			mode, modeErr := strconv.Atoi(fields[1])
			if startErr != nil || endErr != nil || modeErr != nil {
				t.Fatal("invalid blank oracle command or selection")
			}
			doc.Editor.SetSelection(text.Range{Start: start, End: end})
			spans = parseSpans(fields[5])
			length, cursor = len(doc.Editor.String()), doc.Editor.TrackEdits()
			changed = doc.ApplyBlankOperation(BlankOperation(mode))
		case "undo":
			changed = doc != nil && doc.Editor.Undo()
		case "redo":
			changed = doc != nil && doc.Editor.Redo()
		default:
			t.Fatalf("invalid blank oracle phase %q", fields[6])
		}
		if !changed {
			t.Fatalf("blank oracle action refused: %s/%s", fields[0], fields[6])
		}
		edits, next, complete := doc.Editor.EditsSince(cursor, nil)
		if !complete {
			t.Fatal("blank oracle journal is incomplete")
		}
		for _, edit := range edits {
			updated, valid := indicators.Replace(spans, length, edit.Start, edit.Removed, edit.Inserted)
			if !valid {
				t.Fatalf("invalid blank primitive: %+v", edit)
			}
			spans, length = updated, length+edit.Inserted-edit.Removed
		}
		cursor = next
		if doc.Editor.String() != decode(fields[7]) || !slices.Equal(spans, parseSpans(fields[8])) {
			t.Fatalf("blank oracle row %d %s/%s initial=%s: text=%q spans=%+v, want text=%q spans=%s", checked, fields[0], fields[6], fields[5], doc.Editor.String(), spans, decode(fields[7]), fields[8])
		}
		checked++
	}
	if checked != 3579 {
		t.Fatalf("checked %d blank Decoration states, want 3579", checked)
	}
	t.Logf("checked %d original Decoration blank-command states", checked)
}

func TestBlankTabConversionAndCaretMatchSourceLoop(t *testing.T) {
	file, err := os.Open("testdata/scintilla-blank-tabs.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 6
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
		decode := func(value string) string {
			if value == "-" {
				return ""
			}
			decoded, err := hex.DecodeString(value)
			if err != nil {
				t.Fatal(err)
			}
			return string(decoded)
		}
		mode, modeErr := strconv.Atoi(fields[0])
		width, widthErr := strconv.Atoi(fields[2])
		caret, caretErr := strconv.Atoi(fields[3])
		wantCaret, wantErr := strconv.Atoi(fields[5])
		if modeErr != nil || widthErr != nil || caretErr != nil || wantErr != nil {
			t.Fatal("invalid source TAB fixture")
		}
		before, want := decode(fields[1]), decode(fields[4])
		converted, mapped := convertBlankLine(before, width, BlankOperation(mode), caret)
		if converted != want || mapped != wantCaret {
			t.Fatalf("TAB oracle %d mode=%d width=%d input=%q caret=%d: %q/%d, want %q/%d", checked, mode, width, before, caret, converted, mapped, want, wantCaret)
		}
		trigger, convertedTrigger := "\ttrigger", strings.Repeat(" ", width)+"trigger"
		if BlankOperation(mode) != BlankTabToSpace {
			trigger, convertedTrigger = strings.Repeat(" ", width*2)+"trigger", "\t\ttrigger"
			if width == 1 {
				convertedTrigger = "\t trigger"
			}
		}
		doc := New("caret-oracle.txt")
		doc.Editor.SetCode(text.CodeOptions{TabWidth: width, HardTabs: true})
		doc.Editor.Insert(trigger + "\n" + before)
		doc.Editor.ClearHistory()
		doc.Editor.MoveTo(utf8.RuneCountInString(trigger)+1+utf8.RuneCountInString(before[:caret]), false)
		if !doc.ApplyBlankOperation(BlankOperation(mode)) || doc.Editor.String() != convertedTrigger+"\n"+want || doc.Editor.Caret() != utf8.RuneCountInString(convertedTrigger)+1+utf8.RuneCountInString(want[:wantCaret]) {
			t.Fatalf("document TAB oracle %d lost source text/caret after preceding-line edits: %q caret=%d", checked, doc.Editor.String(), doc.Editor.Caret())
		}
		checked++
	}
	if checked != 8100 {
		t.Fatalf("checked %d source TAB/caret states, want 8100", checked)
	}
	t.Logf("checked %d source-transcribed TAB/caret states", checked)
}
