package linemarks

import (
	"encoding/csv"
	"encoding/hex"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLineFactsMapperMatchesOriginalMergeSplitHistory(t *testing.T) {
	file, err := os.Open("testdata/scintilla-marker-line-facts.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 8
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	integer := func(raw string) int {
		t.Helper()
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	decode := func(raw string) string {
		t.Helper()
		if raw == "-" {
			return ""
		}
		value, err := hex.DecodeString(raw)
		if err != nil {
			t.Fatal(err)
		}
		return string(value)
	}
	markers := func(raw string) []int {
		if raw == "-" {
			return nil
		}
		var result []int
		for _, line := range strings.Split(raw, ",") {
			result = append(result, integer(line))
		}
		return result
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
		var edits []LineEdit
		if fields[5] != "-" {
			for _, raw := range strings.Split(fields[5], ",") {
				parts := strings.Split(raw, ":")
				if len(parts) != 8 {
					t.Fatalf("invalid line-facts event %q", raw)
				}
				edits = append(edits, LineEdit{Start: integer(parts[0]), Removed: integer(parts[1]), Inserted: integer(parts[2]), AtLineStart: parts[3] == "1", Line: integer(parts[5]), RemovedLines: integer(parts[6]), InsertedLines: integer(parts[7])})
			}
		}
		got, complete := MapLines(decode(fields[2]), decode(fields[3]), markers(fields[4]), edits, true)
		if !complete || !slices.Equal(got, markers(fields[6])) {
			t.Fatalf("%s/%s: line-facts mapping got %v complete=%v, original wants %v", fields[0], fields[1], got, complete, markers(fields[6]))
		}
		checked++
	}
	if checked != 5460 {
		t.Fatalf("checked %d original line-facts states, want 5460", checked)
	}
	t.Logf("checked %d original line-facts states", checked)
}

func TestLineFactsMappingRejectsPartialAndInvalidSequences(t *testing.T) {
	for _, test := range []struct {
		name     string
		edits    []LineEdit
		complete bool
	}{
		{"gap", nil, false},
		{"negative byte", []LineEdit{{Start: -1}}, true},
		{"negative line", []LineEdit{{Line: -1}}, true},
		{"past source line", []LineEdit{{Line: 2}}, true},
		{"line past byte", []LineEdit{{Line: 1, Start: 0}}, true},
		{"negative LF removal", []LineEdit{{RemovedLines: -1}}, true},
		{"LF removal without bytes", []LineEdit{{RemovedLines: 1}}, true},
		{"LF insertion without bytes", []LineEdit{{InsertedLines: 1}}, true},
		{"too many removed lines", []LineEdit{{Removed: 3, RemovedLines: 2}}, true},
		{"final LF mismatch", []LineEdit{{Start: 1, Removed: 1, Inserted: 1, RemovedLines: 1}}, true},
		{"final byte mismatch", []LineEdit{{Start: 1, Removed: 1, RemovedLines: 1}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			marked := []int{1, 0, 1}
			copyOfMarks := slices.Clone(marked)
			if got, valid := MapLines("a\nb", "a\nb", marked, test.edits, test.complete); valid || got != nil {
				t.Fatalf("invalid line facts accepted: %v valid=%v", got, valid)
			}
			if !slices.Equal(marked, copyOfMarks) {
				t.Fatal("rejected mapping changed source marks")
			}
		})
	}
	if got, valid := MapLines("a\nb", "a\nb", []int{1, 0, 1}, nil, true); !valid || !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("unchanged mapping lost marks: %v %v", got, valid)
	}
}
