package syntax

import (
	"slices"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
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

func TestHighlightSeparatesNumbersFromStringsAndHexData(t *testing.T) {
	for _, tc := range []struct {
		mode, line, number, other string
	}{
		{".json", `{"number": 42, "string": "42"}`, "42", `"42"`},
		{".json5", `{"number": 0x2A, "string": '0x2A'}`, "0x2A", "'0x2A'"},
		{".hex", ":020000001A2BB9", "2B", "1A"},
		{".srec", "S10512341A2B6F", "2B", "1A"},
		{".tek", "%126298000000001A2B", "2B", "1A"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			var highlighter Highlighter
			colors := make(map[string]gfx.Color)
			for _, run := range highlighter.Highlight(tc.mode, tc.line, nil) {
				colors[tc.line[run.Start:run.End]] = run.Color
			}
			if got := colors[tc.number]; got != gfx.RGB(139, 75, 147) {
				t.Errorf("numeric span %q color = %v", tc.number, got)
			}
			if got := colors[tc.other]; got != gfx.RGB(172, 77, 64) {
				t.Errorf("string or odd data span %q color = %v", tc.other, got)
			}
		})
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

func TestJSON5HighlightsKeysWithBlockCommentsBeforeColon(t *testing.T) {
	lexer := lexers.Get("JSON5")
	for _, tc := range []struct {
		name, key, gap string
		comments       []string
	}{
		{"identifier", "answer", "/* : inside */", []string{"/* : inside */"}},
		{"quoted", `'title'`, " \t/**/  /* stars ***/ ", []string{"/**/", "/* stars ***/"}},
		{"escaped Unicode", `caf\u00e9`, "/* \u6807\u7b7e */", []string{"/* \u6807\u7b7e */"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenise := func(source string) []chroma.Token {
				t.Helper()
				iter, err := lexer.Tokenise(nil, source)
				if err != nil {
					t.Fatal(err)
				}
				return iter.Tokens()
			}
			const value = `"value"`
			tokens := tokenise("{" + tc.key + tc.gap + ": " + value + " /* : here */}")
			key := chroma.Token{Type: chroma.NameTag, Value: tc.key}
			if !slices.Contains(tokens, key) || !slices.Contains(tokens, chroma.Token{Type: chroma.LiteralStringDouble, Value: value}) {
				t.Errorf("property or string highlighted incorrectly: %v", tokens)
			}
			for _, value := range tc.comments {
				if !slices.Contains(tokens, chroma.Token{Type: chroma.CommentMultiline, Value: value}) {
					t.Errorf("comment %q was not highlighted separately: %v", value, tokens)
				}
			}
			if slices.Contains(tokenise(tc.key+" /* unterminated :"), key) {
				t.Error("colon inside an unclosed comment highlighted a property")
			}
		})
	}
}
