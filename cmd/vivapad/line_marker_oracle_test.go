package main

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
	"vivapad/internal/document"
)

type sourceLineMarkerEditor interface {
	editLineTargetsWithBookmarks(*document.Document, string, func())
}

func TestLineMarkerForwardProjectionMatchesOriginalCellBuffer(t *testing.T) {
	file, err := os.Open("../../internal/document/testdata/scintilla-line-markers-forward.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 7
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
	parseLines := func(value string) []int {
		t.Helper()
		if value == "-" {
			return nil
		}
		var lines []int
		for _, raw := range strings.Split(value, ",") {
			line, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, line)
		}
		return lines
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
		state := newModel()
		editor, ok := any(state).(sourceLineMarkerEditor)
		if !ok {
			t.Fatal("model has no source line marker projection")
		}
		doc := state.space.Current()
		before := decode(fields[1])
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		state.setBookmarkLines(doc, parseLines(fields[5]))
		type mutation struct {
			at, removed int
			inserted    string
		}
		var changes []mutation
		if fields[4] != "-" {
			for _, raw := range strings.Split(fields[4], ",") {
				parts := strings.Split(raw, ":")
				if len(parts) != 3 {
					t.Fatalf("invalid marker mutation %q", raw)
				}
				at, atErr := strconv.Atoi(parts[0])
				removed, removedErr := strconv.Atoi(parts[1])
				if atErr != nil || removedErr != nil {
					t.Fatal("invalid marker mutation range")
				}
				changes = append(changes, mutation{at: at, removed: removed, inserted: decode(parts[2])})
			}
		}
		editor.editLineTargetsWithBookmarks(doc, fields[3], func() {
			if err := doc.Editor.Transact(func(draft *text.Editor) error {
				for _, change := range changes {
					draft.ReplaceRange(text.Range{Start: draft.RuneIndex(change.at), End: draft.RuneIndex(change.at + change.removed)}, change.inserted)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
		if doc.Editor.String() != decode(fields[2]) || !slices.Equal(state.bookmarkLines(doc), parseLines(fields[6])) {
			t.Fatalf("marker oracle %d %s initial=%s: text=%q markers=%v, want text=%q markers=%s", checked, fields[0], fields[5], doc.Editor.String(), state.bookmarkLines(doc), decode(fields[2]), fields[6])
		}
		checked++
	}
	if checked != 420 {
		t.Fatalf("checked %d source marker cases, want 420", checked)
	}
	t.Logf("checked %d original CellBuffer/LineMarkers forward cases", checked)
}
