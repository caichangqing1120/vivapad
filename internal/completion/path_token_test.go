package completion

import (
	"strings"
	"testing"
)

func TestPathTokenAtCaretExtendsThroughQuotedSpacesAndSuffix(t *testing.T) {
	for _, sample := range []struct {
		line, caretText string
		want            string
	}{
		{`open "folder with spaces/notes.txt" later`, "notes", "folder with spaces/notes.txt"},
		{`open ./src/main.go now`, "main", "./src/main.go"},
		{`C:\\Work\\notes.txt`, "Work", `C:\\Work\\notes.txt`},
		{"notes.txt", "", "notes.txt"},
	} {
		caret := 0
		if sample.caretText != "" {
			caret = len([]rune(sample.line[:strings.Index(sample.line, sample.caretText)+len(sample.caretText)]))
		}
		value, start, end := PathTokenAtCaret(sample.line, caret)
		if value != sample.want || start < 0 || end > len([]rune(sample.line)) || end <= start {
			t.Fatalf("path token at %d in %q = %q (%d:%d), want %q", caret, sample.line, value, start, end, sample.want)
		}
	}
	if value, _, _ := PathTokenAtCaret("first\nsecond.txt", len([]rune("first\nsec"))); value != "second.txt" {
		t.Fatalf("path token crossed lines: %q", value)
	}
}
