package linemarks

import (
	"bufio"
	"encoding/hex"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestMapOriginalMarkerEffects(t *testing.T) {
	tests := []struct {
		name, before, after string
		marked, want        []int
		edits               []Edit
	}{
		{"whole delete clears", "a\nb", "", []int{0, 1}, nil, []Edit{{Start: 0, Removed: 3, LineStart: true}}},
		{"partial merge retains", "a\nb\nc", "ac", []int{1, 2}, []int{0}, []Edit{{Start: 1, Removed: 3}}},
		{"start insertion shifts", "a\nb", "x\ny\na\nb", []int{0, 1}, []int{2, 3}, []Edit{{Start: 0, Inserted: 4, LineStart: true, InsertedLF: true}}},
		{"middle insertion stays", "ab", "a\nb", []int{0}, []int{0}, []Edit{{Start: 1, Inserted: 1, InsertedLF: true}}},
		{"undo does not restore a split marker", "ab", "a\nb", []int{0}, []int{0}, []Edit{{Start: 1, Inserted: 1, InsertedLF: true}}},
		{"equal replacement clears", "a\nb", "a\nb", []int{0, 1}, nil, []Edit{{Start: 0, Removed: 3, Inserted: 3, InsertedLF: true, LineStart: true}}},
		{"unicode byte offsets", "\u732b\n\u00e9", "\u732b\u00e9", []int{1}, []int{0}, []Edit{{Start: 3, Removed: 1}}},
		{"unordered initial marks", "a\nb\nc", "a\nb\nc", []int{2, 0, 2}, []int{0, 2}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := slices.Clone(test.marked)
			got, complete := Map(test.before, test.after, test.marked, test.edits, true)
			if !complete || !slices.Equal(got, test.want) {
				t.Fatalf("Map = %v, %v; want %v, true", got, complete, test.want)
			}
			if !slices.Equal(test.marked, original) {
				t.Fatal("mapping mutated the source marks")
			}
		})
	}
}

func TestMapNeedsActualTransientLineContext(t *testing.T) {
	for _, firstLF := range []bool{false, true} {
		edits := []Edit{
			{Start: 0, Inserted: 1, LineStart: true, InsertedLF: firstLF},
			{Start: 1, Inserted: 1, LineStart: firstLF, InsertedLF: true},
			{Start: 0, Removed: 1, LineStart: true},
		}
		want := []int{0}
		if firstLF {
			want[0] = 1
		}
		got, complete := Map("ab", "\nab", []int{0}, edits, true)
		if !complete || !slices.Equal(got, want) {
			t.Fatalf("first transient LF=%v: got %v, %v; want %v", firstLF, got, complete, want)
		}
	}
}

func TestMapRejectsMissingOrInvalidMetadata(t *testing.T) {
	if got, complete := Map("a\nb", "ab\n", []int{1}, nil, true); complete || got != nil {
		t.Fatalf("changed text without metadata mapped to %v, %v", got, complete)
	}
	for _, test := range []struct {
		name     string
		marked   []int
		edits    []Edit
		complete bool
	}{
		{"journal gap", []int{0}, nil, false},
		{"negative mark", []int{-1}, nil, true},
		{"past last line", []int{1}, nil, true},
		{"negative position", []int{0}, []Edit{{Start: -1}}, true},
		{"negative removal", []int{0}, []Edit{{Removed: -1}}, true},
		{"negative insertion", []int{0}, []Edit{{Inserted: -1}}, true},
		{"past source", []int{0}, []Edit{{Start: 4}}, true},
		{"removal past source", []int{0}, []Edit{{Start: 1, Removed: 3}}, true},
		{"inconsistent LF", []int{0}, []Edit{{InsertedLF: true}}, true},
		{"different final length", []int{0}, []Edit{{Inserted: 1}}, true},
		{"length overflow", []int{0}, []Edit{{Inserted: int(^uint(0) >> 1)}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, complete := Map("abc", "abc", test.marked, test.edits, test.complete)
			if complete || got != nil {
				t.Fatalf("invalid sequence mapped to %v, %v", got, complete)
			}
		})
	}
}

func TestMapMatchesOriginalCellBufferHistory(t *testing.T) {
	input, err := os.Open("testdata/scintilla-marker-history.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
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
	integer := func(value string) int {
		t.Helper()
		parsed, err := strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	markers := func(value string) []int {
		if value == "-" {
			return nil
		}
		var lines []int
		for _, line := range strings.Split(value, ",") {
			lines = append(lines, integer(line))
		}
		return lines
	}
	rows, phases := 0, map[string]int{}
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		row := scanner.Text()
		if strings.HasPrefix(row, "#") || strings.HasPrefix(row, "case\t") {
			continue
		}
		fields := strings.Split(row, "\t")
		if len(fields) != 7 {
			t.Fatalf("bad fixture row: %s", row)
		}
		var edits []Edit
		if fields[5] != "-" {
			for _, event := range strings.Split(fields[5], ",") {
				parts := strings.Split(event, ":")
				if len(parts) != 5 || parts[3] != "0" && parts[3] != "1" || parts[4] != "0" && parts[4] != "1" {
					t.Fatalf("bad event: %s", event)
				}
				edits = append(edits, Edit{Start: integer(parts[0]), Removed: integer(parts[1]), Inserted: integer(parts[2]), LineStart: parts[3] == "1", InsertedLF: parts[4] == "1"})
			}
		}
		got, complete := Map(decode(fields[2]), decode(fields[3]), markers(fields[4]), edits, true)
		if !complete || !slices.Equal(got, markers(fields[6])) {
			t.Fatalf("%s/%s: got %v, %v; want %v", fields[0], fields[1], got, complete, markers(fields[6]))
		}
		rows++
		phases[fields[1]]++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != 2652 || phases["forward"] != 884 || phases["undo"] != 884 || phases["redo"] != 884 {
		t.Fatalf("incomplete fixture: rows=%d phases=%v", rows, phases)
	}
}
