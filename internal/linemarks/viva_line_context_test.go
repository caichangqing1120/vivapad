package linemarks

import (
	"bufio"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

type historyMutation struct {
	start, removed int
	inserted       string
}

func historyValue(t *testing.T, value string) string {
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

func historyInteger(t *testing.T, value string) int {
	t.Helper()
	number, err := strconv.Atoi(value)
	if err != nil {
		t.Fatal(err)
	}
	return number
}

func historyMarks(t *testing.T, value string) []int {
	t.Helper()
	if value == "-" {
		return nil
	}
	var marks []int
	for _, marked := range strings.Split(value, ",") {
		marks = append(marks, historyInteger(t, marked))
	}
	return marks
}

func historyMutations(t *testing.T) map[string][]historyMutation {
	t.Helper()
	input, err := os.Open(filepath.Join("..", "document", "testdata", "scintilla-line-markers-forward.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	scenarios := make(map[string][]historyMutation)
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 7 || fields[0] == "case" {
			continue
		}
		if _, exists := scenarios[fields[0]]; exists {
			continue
		}
		var mutations []historyMutation
		if fields[4] != "-" {
			for _, encoded := range strings.Split(fields[4], ",") {
				parts := strings.Split(encoded, ":")
				if len(parts) != 3 {
					t.Fatal("invalid original mutation", encoded)
				}
				mutations = append(mutations, historyMutation{historyInteger(t, parts[0]), historyInteger(t, parts[1]), historyValue(t, parts[2])})
			}
		}
		scenarios[fields[0]] = mutations
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	scenarios["transient-text"] = []historyMutation{{0, 0, "x"}, {1, 0, "\n"}, {0, 1, ""}}
	scenarios["transient-lf"] = []historyMutation{{0, 0, "\n"}, {1, 0, "\n"}, {0, 1, ""}}
	scenarios["whole-equal"] = []historyMutation{{0, 3, "a\nb"}}
	scenarios["whole-delete"] = []historyMutation{{0, 3, ""}}
	scenarios["from-empty"] = []historyMutation{{0, 0, "a\nb"}}
	scenarios["unicode-merge"] = []historyMutation{{3, 1, ""}, {6, 0, "\n"}}
	return scenarios
}

func TestVivaLineContextMatchesOriginalMarkerHistory(t *testing.T) {
	checkVivaMarkerHistory(t, false)
}

func TestVivaLineFactsMatchOriginalMarkerHistory(t *testing.T) {
	checkVivaMarkerHistory(t, true)
}

func checkVivaMarkerHistory(t *testing.T, lineFacts bool) {
	t.Helper()
	scenarios := historyMutations(t)
	fixture, expectedRows := "testdata/scintilla-marker-history.tsv", 2652
	if lineFacts {
		fixture, expectedRows = "testdata/scintilla-marker-line-facts.tsv", 5460
	}
	input, err := os.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	var editor *text.Editor
	var cursor uint64
	active, rows := "", 0
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	for scanner.Scan() {
		row := scanner.Text()
		if strings.HasPrefix(row, "#") || strings.HasPrefix(row, "case\t") {
			continue
		}
		fields := strings.Split(row, "\t")
		expectedFields := 7
		if lineFacts {
			expectedFields = 8
		}
		if len(fields) != expectedFields {
			t.Fatal("invalid history row", row)
		}
		before, after := historyValue(t, fields[2]), historyValue(t, fields[3])
		if fields[1] == "forward" {
			editor = text.NewEditor(before)
			editor.SetSavePoint()
			cursor = editor.TrackLineEdits()
			active = fields[0]
		}
		if active != fields[0] || editor.String() != before {
			t.Fatalf("%s/%s: source history is not at %q", fields[0], fields[1], before)
		}
		switch fields[1] {
		case "forward":
			name, _, _ := strings.Cut(fields[0], "/")
			mutations, exists := scenarios[name]
			if lineFacts {
				exists, mutations = true, nil
				if fields[7] != "-" {
					for _, raw := range strings.Split(fields[7], ",") {
						parts := strings.Split(raw, ":")
						if len(parts) != 3 {
							t.Fatal("invalid rich mutation payload", raw)
						}
						mutations = append(mutations, historyMutation{historyInteger(t, parts[0]), historyInteger(t, parts[1]), historyValue(t, parts[2])})
					}
				}
			}
			if !exists {
				t.Fatal("missing original mutation payloads", name)
			}
			if err := editor.Transact(func(draft *text.Editor) error {
				for _, mutation := range mutations {
					draft.ReplaceRange(text.Range{Start: draft.RuneIndex(mutation.start), End: draft.RuneIndex(mutation.start + mutation.removed)}, mutation.inserted)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		case "undo":
			editor.Undo()
		case "redo":
			editor.Redo()
		default:
			t.Fatal("unknown phase", fields[1])
		}
		if editor.String() != after {
			t.Fatalf("%s/%s: text=%q, want %q", fields[0], fields[1], editor.String(), after)
		}
		actual, next, complete := editor.LineEditsSince(cursor, nil)
		cursor = next
		var edits []Edit
		var exact []LineEdit
		for _, edit := range actual {
			edits = append(edits, Edit{Start: edit.Start, Removed: edit.Removed, Inserted: edit.Inserted, LineStart: edit.AtLineStart, InsertedLF: edit.InsertedLF})
			exact = append(exact, LineEdit{Start: edit.Start, Removed: edit.Removed, Inserted: edit.Inserted, Line: edit.Line, RemovedLines: edit.RemovedLines, InsertedLines: edit.InsertedLines, AtLineStart: edit.AtLineStart})
		}
		got, valid := Map(before, after, historyMarks(t, fields[4]), edits, complete)
		if lineFacts {
			got, valid = MapLines(before, after, historyMarks(t, fields[4]), exact, complete)
		}
		if !valid || !slices.Equal(got, historyMarks(t, fields[6])) {
			t.Fatalf("%s/%s: actual Viva metadata mapped %v, %v; want %v", fields[0], fields[1], got, valid, historyMarks(t, fields[6]))
		}
		rows++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != expectedRows {
		t.Fatal("incomplete original history fixture", rows)
	}
}
