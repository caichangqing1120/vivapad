package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/vivaui/viva/components"
)

func TestJSON5ContinuedStringsEndBeforeFollowingProperties(t *testing.T) {
	for _, quote := range []string{`'`, `"`} {
		t.Run(quote, func(t *testing.T) {
			h := NewJSON5Highlighter(&Highlighter{})
			first := "name: " + quote + `a\`
			middle := `/* inside the string */\`
			last := "end" + quote + ", enabled: true"
			paint := h.ForText(strings.Join([]string{first, middle, last}, "\n"))
			valueColor, _ := tokenColor(chroma.LiteralString)
			keyColor, _ := tokenColor(chroma.NameTag)
			// A viewport may start at the closing line without painting the opener.
			runs := paint(".json5", 2, last, nil)
			if len(runs) != 3 || runs[0] != (components.CodeEditorRun{Start: 0, End: 4, Color: valueColor}) ||
				runs[1] != (components.CodeEditorRun{Start: 6, End: 13, Color: keyColor}) {
				t.Fatalf("continuation and following properties were not distinguished: %v", runs)
			}
			if runs := paint(".json5", 1, middle, nil); len(runs) != 1 || runs[0].Color != valueColor ||
				runs[0].Start != 0 || runs[0].End != len(middle) {
				t.Fatalf("comment markers inside a continued string colored it incorrectly: %v", runs)
			}
			if runs := paint(".json5", 0, first, nil); len(runs) != 2 || runs[1].Color != valueColor || runs[1].End != len(first) {
				t.Fatalf("opening line lost the continued string: %v", runs)
			}
		})
	}
}

func TestJSON5ContextUsesOversizedLineDelimitersWithoutColoringThatLine(t *testing.T) {
	h := NewJSON5Highlighter(&Highlighter{})
	oversized := strings.Repeat(" ", components.CodeEditorMaxHighlightedLine) + "/*"
	line := "still commented"
	paint := h.ForText(oversized + "\n" + line)
	if runs := paint(".json5", 0, oversized, nil); len(runs) != 0 {
		t.Fatalf("oversized line was highlighted: %v", runs)
	}
	commentColor, _ := tokenColor(chroma.Comment)
	if runs := paint(".json5", 1, line, nil); len(runs) != 1 || runs[0].Color != commentColor || runs[0].End != len(line) {
		t.Fatalf("following line lost the block comment context: %v", runs)
	}
	paint = h.ForText("// note\u2028/*\n" + line)
	if runs := paint(".json5", 1, line, nil); len(runs) != 1 || runs[0].Color != commentColor {
		t.Fatalf("block opener after a JSON5 line separator was ignored: %v", runs)
	}
}

func TestJSON5ContextReleasesLargeDiscardedSuffix(t *testing.T) {
	const far = 32768
	first := "/*\n\u4f60\n"
	h := NewJSON5Highlighter(&Highlighter{})
	paint := h.ForText(first + strings.Repeat("long comment\n", far) + "*/")
	commentColor, _ := tokenColor(chroma.Comment)
	if runs := paint(".json5", far+1, "long comment", nil); len(runs) != 1 || runs[0].Color != commentColor {
		t.Fatalf("deep viewport lost its comment state: %v", runs)
	}
	if len(h.lines) <= far {
		t.Fatal("the test did not populate the large index")
	}

	// Keep the opener and Unicode comment text, but replace the bulk of the
	// document. The previous scroll must not reserve an index for those lines.
	last := "*/ {answer: 42}"
	paint = h.ForText(first + last)
	if cap(h.lines) >= far/4 {
		t.Errorf("shortened document still reserves %d line entries", cap(h.lines))
	}
	keyColor, _ := tokenColor(chroma.NameTag)
	if runs := paint(".json5", 2, last, nil); len(runs) != 3 || runs[0] != (components.CodeEditorRun{Start: 0, End: 2, Color: commentColor}) ||
		runs[1] != (components.CodeEditorRun{Start: 4, End: 10, Color: keyColor}) {
		t.Fatalf("deleting the suffix lost the kept comment context or colored past its closer: %v", runs)
	}
}
