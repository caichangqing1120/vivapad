package indicators

import (
	"bufio"
	"encoding/csv"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func parseSpans(t *testing.T, value string) []text.Span {
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
			t.Fatalf("invalid fixture span %q", pair)
		}
		spans = append(spans, text.Span{Start: first, End: last})
	}
	return spans
}

func TestReplaceMatchesPinnedScintillaDecorationOracle(t *testing.T) {
	fixture, err := os.Open("testdata/scintilla-edits.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	scanner := bufio.NewScanner(fixture)
	checked := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "length\t") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 6 {
			t.Fatalf("invalid oracle row %q", line)
		}
		var numbers [4]int
		for index, field := range []int{0, 2, 3, 4} {
			numbers[index], err = strconv.Atoi(fields[field])
			if err != nil {
				t.Fatal(err)
			}
		}
		before, want := parseSpans(t, fields[1]), parseSpans(t, fields[5])
		got, valid := Replace(before, numbers[0], numbers[1], numbers[2], numbers[3])
		if !valid || !slices.Equal(got, want) {
			t.Fatalf("oracle case %d (%s): got %+v valid=%v, want %+v", checked, line, got, valid, want)
		}
		checked++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if checked != 1080 {
		t.Fatalf("checked %d source cases, want 1080", checked)
	}
}

func TestReplaceRejectsInvalidMutationWithoutChangingInput(t *testing.T) {
	spans := []text.Span{{Start: 2, End: 5}}
	for _, edit := range [][3]int{{-1, 0, 1}, {9, 0, 1}, {7, 2, 0}, {1, -1, 0}, {1, 0, -1}} {
		if _, valid := Replace(spans, 8, edit[0], edit[1], edit[2]); valid {
			t.Errorf("invalid edit accepted: %v", edit)
		}
	}
	if !slices.Equal(spans, []text.Span{{Start: 2, End: 5}}) {
		t.Fatal("invalid edit mutated source spans")
	}
}

func TestReplaceMatchesPinnedTargetForwardUndoRedoSequences(t *testing.T) {
	file, err := os.Open("testdata/scintilla-target-sequences.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 7
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
		var numbers [4]int
		for index, field := range []int{1, 3, 4, 5} {
			numbers[index], err = strconv.Atoi(fields[field])
			if err != nil {
				t.Fatal(err)
			}
		}
		got, valid := Replace(parseSpans(t, fields[2]), numbers[0], numbers[1], numbers[2], numbers[3])
		if want := parseSpans(t, fields[6]); !valid || !slices.Equal(got, want) {
			t.Fatalf("%s source step %d = %+v, want %+v", fields[0], checked, got, want)
		}
		checked++
	}
	if checked != 15 {
		t.Fatalf("checked %d target sequence steps, want 15", checked)
	}
}
