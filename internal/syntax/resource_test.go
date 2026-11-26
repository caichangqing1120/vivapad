package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func TestResourceHighlighterSeesCommentOpenerBeforeViewport(t *testing.T) {
	const opener = "#define TITLE /* opening\n"
	const skipped = 200
	const row = "MENU */ DIALOGEX"
	value := opener + strings.Repeat("comment text\n", skipped) + row
	var shared Highlighter
	highlighter := NewResourceHighlighter(&shared)
	check := func(value string, kind chroma.TokenType) {
		t.Helper()
		runs := highlighter.ForText(value)(".rc", skipped+1, row, nil)
		first, _ := tokenColor(kind)
		keyword, _ := tokenColor(chroma.Keyword)
		if len(runs) == 0 || runs[0].Start != 0 || runs[0].End < len("MENU") || runs[0].Color != first {
			t.Errorf("first token after skipping offscreen lines = %+v, want %v", runs, kind)
		}
		last := strings.Index(row, "DIALOGEX")
		if len(runs) == 0 || runs[len(runs)-1].Start != last || runs[len(runs)-1].End != len(row) || runs[len(runs)-1].Color != keyword {
			t.Errorf("keyword after the closer = %+v", runs)
		}
	}
	check(value, chroma.CommentMultiline)
	check(strings.Replace(value, "/*", "  ", 1), chroma.Keyword)
}
