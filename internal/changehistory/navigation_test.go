package changehistory

import (
	"encoding/csv"
	"encoding/hex"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func historyFixtureReader(t *testing.T, path string, fields int) *csv.Reader {
	t.Helper()
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { input.Close() })
	reader := csv.NewReader(input)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', fields
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	return reader
}

func TestLineMaskMatchesUnmodifiedOriginalDocument(t *testing.T) {
	checkLineMasksFixture(t, "testdata/scintilla-change-history.tsv", "testdata/scintilla-change-lines.tsv", 2293)
}

func TestLineMaskMatchesSpanCorrectedOriginalDocument(t *testing.T) {
	checkLineMasksFixture(t, "testdata/scintilla-change-history-span-fixed.tsv", "testdata/scintilla-change-lines-span-fixed.tsv", 8299)
}

func checkLineMasksFixture(t *testing.T, historyPath, linesPath string, wanted int) {
	t.Helper()
	history, lines := historyFixtureReader(t, historyPath, 8), historyFixtureReader(t, linesPath, 3)
	var tracker *Tracker
	active, checked, lineQueries := "", 0, 0
	for {
		row, err := history.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		expected, err := lines.Read()
		if err != nil || expected[0] != row[0] || expected[1] != row[1] {
			t.Fatalf("line fixture does not match original %s/%s: %v, %v", row[0], row[1], expected, err)
		}
		if active != row[0] {
			tracker, err = New(0)
			if err != nil {
				t.Fatal(err)
			}
			active = row[0]
		}
		if row[3] != "-" {
			for _, event := range strings.Split(row[3], ";") {
				if err := tracker.Apply(referenceEvent(t, event)); err != nil {
					t.Fatalf("%s/%s: %v", row[0], row[1], err)
				}
			}
		}
		value := ""
		if row[2] != "-" {
			decoded, err := hex.DecodeString(row[2])
			if err != nil {
				t.Fatal(err)
			}
			value = string(decoded)
		}
		masks, contents := strings.Split(expected[2], ","), strings.SplitAfter(value, "\n")
		if len(masks) != len(contents) {
			t.Fatal("incomplete line mask fixture", row[0], row[1])
		}
		start := 0
		for line, content := range contents {
			next := start + len(content)
			end := next
			if strings.HasSuffix(content, "\n") {
				end--
			}
			mask, err := tracker.LineMask(start, end, next)
			if err != nil || int(mask) != referenceInteger(t, masks[line]) {
				t.Fatalf("%s/%s line=%d bounds=%d,%d,%d: mask=%d, want %s; %v", row[0], row[1], line, start, end, next, mask, masks[line], err)
			}
			start = next
			lineQueries++
		}
		checked++
	}
	if _, err := lines.Read(); err != io.EOF || checked != wanted {
		t.Fatalf("line fixture size mismatch: %d, want %d; remaining read: %v", checked, wanted, err)
	}
	t.Logf("matched %d original states and %d line masks", checked, lineQueries)
}

func TestLineMaskOwnsInsertedEOLAndDeletedEOF(t *testing.T) {
	for _, test := range []struct {
		name             string
		length           int
		event            Event
		start, end, next int
		mask             uint8
	}{
		{"inserted LF belongs to preceding line", 2, Event{Kind: Insert, Position: 1, Length: 1, CollectingUndo: true}, 0, 1, 2, 4},
		{"inserted LF does not mark following line", 2, Event{Kind: Insert, Position: 1, Length: 1, CollectingUndo: true}, 2, 3, 3, 0},
		{"deletion at EOF marks empty last line", 3, Event{Kind: Delete, Position: 2, Length: 1}, 2, 2, 2, 4},
		{"deletion at line end included", 4, Event{Kind: Delete, Position: 1, Length: 1}, 0, 1, 2, 4},
		{"deletion in CRLF interior excluded", 5, Event{Kind: Delete, Position: 2, Length: 1}, 0, 1, 3, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			tracker, err := New(test.length)
			if err != nil {
				t.Fatal(err)
			}
			if err := tracker.Apply(test.event); err != nil {
				t.Fatal(err)
			}
			before := queryTracker(tracker)
			mask, err := tracker.LineMask(test.start, test.end, test.next)
			if err != nil || mask != test.mask {
				t.Fatalf("mask=%d, want %d; %v", mask, test.mask, err)
			}
			if !reflect.DeepEqual(queryTracker(tracker), before) {
				t.Fatal("line query mutated metadata")
			}
		})
	}
}

func TestLineMaskRefusesInvalidBoundsWithoutMutation(t *testing.T) {
	tracker, err := New(4)
	if err != nil {
		t.Fatal(err)
	}
	if err := tracker.Apply(Event{Kind: Insert, Position: 1, Length: 1, CollectingUndo: true}); err != nil {
		t.Fatal(err)
	}
	before := queryTracker(tracker)
	for _, bounds := range [][3]int{{-1, 0, 1}, {2, 1, 3}, {1, 3, 2}, {1, 4, 6}, {6, 6, 6}} {
		if _, err := tracker.LineMask(bounds[0], bounds[1], bounds[2]); err == nil {
			t.Fatalf("accepted invalid bounds %v", bounds)
		}
	}
	if !reflect.DeepEqual(queryTracker(tracker), before) {
		t.Fatal("invalid line query mutated metadata")
	}
}

func TestChangedLineMatchesOriginalNavigation(t *testing.T) {
	reader := historyFixtureReader(t, "testdata/npp-change-navigation.tsv", 5)
	checked := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		count, bits := referenceInteger(t, row[0]), referenceInteger(t, row[1])
		current, previous, wanted := referenceInteger(t, row[2]), row[3] == "1", referenceInteger(t, row[4])
		got, found := ChangedLine(current, count, previous, func(line int) bool {
			if line < 0 || line >= count {
				t.Fatalf("queried out-of-range line %d for %v", line, row)
			}
			return bits&(1<<line) != 0
		})
		if got != wanted || found != (wanted >= 0) {
			t.Fatalf("original %v: got %d,%v", row, got, found)
		}
		checked++
	}
	if checked != 16388 {
		t.Fatalf("matched %d original navigation cases, want 16388", checked)
	}
	t.Logf("matched %d original navigation cases", checked)
}

func TestChangedLineRejectsInvalidScopeBeforeQuerying(t *testing.T) {
	for _, scope := range [][2]int{{0, 0}, {-1, 2}, {2, 2}, {0, -1}} {
		for _, previous := range []bool{false, true} {
			line, found := ChangedLine(scope[0], scope[1], previous, func(int) bool {
				t.Fatal("invalid scope queried line markers")
				return false
			})
			if line != -1 || found {
				t.Fatalf("invalid scope %v returned %d,%v", scope, line, found)
			}
		}
	}
	if line, found := ChangedLine(0, 1, false, nil); line != -1 || found {
		t.Fatal("nil query produced a target")
	}
}
