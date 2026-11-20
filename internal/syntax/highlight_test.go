package syntax

import (
	"slices"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/vivaui/viva/gfx"
)

func TestGoAndJSONHighlightUseRealLexers(t *testing.T) {
	var highlighter Highlighter
	goRuns := highlighter.Highlight(".go", "func main() {", nil)
	if len(goRuns) == 0 || goRuns[0].Start != 0 || goRuns[0].End != 4 {
		t.Fatalf("Go keyword runs = %+v", goRuns)
	}
	jsonRuns := highlighter.Highlight(".json", `{"name": 42}`, nil)
	if len(jsonRuns) < 2 {
		t.Fatalf("JSON value runs = %+v", jsonRuns)
	}
	if plain := highlighter.Highlight(".txt", "func main() {", nil); len(plain) != 0 {
		t.Fatalf("plain text was highlighted: %+v", plain)
	}
}

func TestHighlightCacheDoesNotReturnStaleLine(t *testing.T) {
	var highlighter Highlighter
	first := highlighter.Highlight(".go", "func one() {}", nil)
	second := highlighter.Highlight(".go", "func two() {}", nil)
	if len(first) == 0 || len(second) == 0 {
		t.Fatalf("missing runs: %+v / %+v", first, second)
	}
	if repeated := highlighter.Highlight(".go", "func one() {}", nil); len(repeated) != len(first) {
		t.Fatalf("cached runs changed: %+v / %+v", first, repeated)
	}
}

func TestJSON5HighlightsExtendedTokensWithoutChangingJSON(t *testing.T) {
	var highlighter Highlighter
	const line = `{answer: +0x2A, name: 'demo', "quoted": -.5, more: +Infinity, missing: -NaN, ok: true,} // note`
	type part struct {
		text  string
		color gfx.Color
	}
	painted := func(text string, kind chroma.TokenType) part {
		color, _ := tokenColor(kind)
		return part{text, color}
	}
	ranges := func(mode, source string) []part {
		var got []part
		for _, run := range highlighter.Highlight(mode, source, nil) {
			got = append(got, part{source[run.Start:run.End], run.Color})
		}
		return got
	}
	want := []part{
		painted("answer", chroma.NameTag), painted("+0x2A", chroma.LiteralNumber),
		painted("name", chroma.NameTag), painted("'demo'", chroma.LiteralString),
		painted(`"quoted"`, chroma.NameTag), painted("-.5", chroma.LiteralNumber),
		painted("more", chroma.NameTag), painted("+Infinity", chroma.LiteralNumber),
		painted("missing", chroma.NameTag), painted("-NaN", chroma.LiteralNumber),
		painted("ok", chroma.NameTag), painted("true", chroma.KeywordConstant), painted("// note", chroma.Comment),
	}
	for _, mode := range []string{".json5", ".jsonc"} {
		if got := ranges(mode, line); !slices.Equal(got, want) {
			t.Errorf("%s: highlight runs = %v; want %v", mode, got, want)
		}
		const escapes = `/* context */ caf\u00e9: 'a\'b',`
		wantEscapes := []part{painted("/* context */", chroma.Comment), painted(`caf\u00e9`, chroma.NameTag), painted(`'a\'b'`, chroma.LiteralString)}
		if got := ranges(mode, escapes); !slices.Equal(got, wantEscapes) {
			t.Errorf("%s: escaped token highlights = %v; want %v", mode, got, wantEscapes)
		}
	}
	if ordinary := ranges(".json", line); slices.Contains(ordinary, painted("+0x2A", chroma.LiteralNumber)) {
		t.Fatalf("plain JSON inherited the JSON5 number rule: %v", ordinary)
	}
}
