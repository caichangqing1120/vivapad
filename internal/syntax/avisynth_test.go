package syntax

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestAviSynthWordGroupsSurviveCRLFCheckout(t *testing.T) {
	lf := strings.ReplaceAll(avsWordData, "\r\n", "\n")
	if !maps.Equal(parseAVSWords(lf), parseAVSWords(strings.ReplaceAll(lf, "\n", "\r\n"))) {
		t.Fatal("CRLF changed the AviSynth word groups")
	}
}

func TestAviSynthTokenKindsAndMultilineState(t *testing.T) {
	const source = "GLOBAL clip = BlAnKcLiP()\r\n/* outer\r\n/* nested */ still comment\r\n*/ RETURN\n" +
		"[* outer\r[* nested *] still bracket comment\r*]\r\n" +
		"\"\"\"first\n{ # not code\n\"\"\" + .5\n\"ordinary # text\" # comment\nAbs(2) FrameCount AddGrain"
	lexer := lexers.Get("AviSynth")
	if lexer == nil || lexer.Config().Name != "AviSynth" {
		t.Fatal("AviSynth lexer is missing")
	}
	next, err := lexer.Tokenise(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	var decoded strings.Builder
	styles := make([]chroma.TokenType, 0, len(source))
	for token := next(); token != chroma.EOF; token = next() {
		decoded.WriteString(token.Value)
		for range len(token.Value) {
			styles = append(styles, token.Type)
		}
	}
	if decoded.String() != source {
		t.Fatal("tokenisation changed source bytes")
	}
	for _, tc := range []struct {
		word string
		kind chroma.TokenType
	}{
		{"GLOBAL", chroma.Keyword}, {"BlAnKcLiP", chroma.NameFunction},
		{"/* nested */ still comment", chroma.CommentMultiline}, {"RETURN", chroma.Keyword},
		{"[* nested *] still bracket comment", chroma.CommentMultiline},
		{"{ # not code", chroma.LiteralStringDouble}, {".5", chroma.LiteralNumber},
		{"\"ordinary # text\"", chroma.LiteralStringDouble}, {"# comment", chroma.CommentSingle},
		{"Abs", chroma.NameBuiltin}, {"FrameCount", chroma.NameProperty}, {"AddGrain", chroma.NameDecorator},
	} {
		start := strings.Index(source, tc.word)
		for _, kind := range styles[start : start+len(tc.word)] {
			if kind != tc.kind {
				t.Errorf("%q classified as %v, want %v", tc.word, kind, tc.kind)
				break
			}
		}
	}
}

func TestAviSynthFoldsCodeAndOuterComments(t *testing.T) {
	const source = "function demo() {\r\n/* outer\n/* nested */ }\n*/\n[* bracket\n[* { *]\n*]\n" +
		"\"\"\"text\n}\n\"\"\"\n# }\nreturn 1\n}\n"
	want := []FoldRegion{{Header: 0, LastLine: 12, Level: 1}, {Header: 1, LastLine: 3, Level: 2}, {Header: 4, LastLine: 6, Level: 2}}
	for _, ext := range []string{".avs", ".AVSI"} {
		if got := FoldRegions(ext, source); !slices.Equal(got, want) {
			t.Errorf("%s folds = %+v, want %+v", ext, got, want)
		}
	}
}
