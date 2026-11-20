package syntax

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestCSoundTokenKindsAndContinuedComments(t *testing.T) {
	const source = "sr = 48000\r\ninstr 1\naSignal oscil .5, p4\rkLevel init 0.25\r\n" +
		"iPitch = 440\ngMix = aSignal\nAdd add FLbutton flbutton\n; note \\\r\ncontinued caff\u00e8\nendin"
	lexer := lexers.Get("CSound")
	if lexer == nil || lexer.Config().Name != "CSound" {
		t.Fatal("CSound lexer is missing")
	}
	next, err := lexer.Tokenise(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	var decoded strings.Builder
	kinds := make([]chroma.TokenType, 0, len(source))
	for token := next(); token != chroma.EOF; token = next() {
		decoded.WriteString(token.Value)
		for range len(token.Value) {
			kinds = append(kinds, token.Type)
		}
	}
	if decoded.String() != source {
		t.Fatal("tokenisation changed source bytes")
	}
	for _, tc := range []struct {
		word string
		kind chroma.TokenType
	}{
		{"sr", chroma.KeywordDeclaration}, {"48000", chroma.LiteralNumber}, {"instr", chroma.Keyword},
		{"aSignal", chroma.NameVariable}, {"oscil", chroma.Keyword}, {".5", chroma.LiteralNumber},
		{"p4", chroma.NameVariable}, {"kLevel", chroma.NameVariable}, {"iPitch", chroma.NameVariable},
		{"gMix", chroma.NameVariableGlobal}, {"Add", chroma.Keyword}, {"add", chroma.NameVariable},
		{"FLbutton", chroma.Keyword}, {"flbutton", chroma.Text}, {"; note", chroma.CommentSingle},
		{"continued caff\u00e8", chroma.CommentSingle}, {"endin", chroma.Keyword},
	} {
		start := strings.Index(source, tc.word)
		for _, kind := range kinds[start : start+len(tc.word)] {
			if kind != tc.kind {
				t.Errorf("%q classified as %v, want %v", tc.word, kind, tc.kind)
				break
			}
		}
	}
}

func TestCSoundFoldsPairInstrumentOpcodesOutsideComments(t *testing.T) {
	const source = "; instr 9\r\ninstr 1\r  a1 oscil .5, p4\n  ; endin \\\r\nendin\r\nendin\r\n" +
		"endin\ninstr 2\n  out a1\nendin\ninstr 3\n"
	want := []FoldRegion{{Header: 1, LastLine: 5, Level: 1}, {Header: 7, LastLine: 9, Level: 1}}
	for _, ext := range []string{".orc", ".SCO", ".csd"} {
		if got := FoldRegions(ext, source); !slices.Equal(got, want) {
			t.Errorf("%s folds = %+v, want %+v", ext, got, want)
		}
	}
}
