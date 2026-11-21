package syntax

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestFreeBasicWordsLiteralsAndMultilineComments(t *testing.T) {
	const source = "  #INCLUDE \"demo.bi\"\r\nstart: DiM value As Integer = &hA5\r" +
		"print &B10, &o17, .5e-2, \"gr\u00fc\"\"n\" ' short\r\nEnviron$(\"key\")\n/' outer\r\nEnd Sub\n'/ ReM line comment\n"
	lexer := lexers.Get("FreeBasic")
	if lexer == nil || lexer.Config().Name != "FreeBasic" {
		t.Fatal("FreeBasic lexer is missing")
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
		{"#INCLUDE", chroma.CommentPreproc}, {"\"demo.bi\"", chroma.LiteralStringDouble},
		{"start:", chroma.NameLabel}, {"DiM", chroma.Keyword}, {"Integer", chroma.Keyword},
		{"&hA5", chroma.LiteralNumberHex}, {"&B10", chroma.LiteralNumberBin}, {"&o17", chroma.LiteralNumberOct},
		{".5e-2", chroma.LiteralNumber}, {"\"gr\u00fc\"\"n\"", chroma.LiteralStringDouble}, {"' short", chroma.CommentSingle},
		{"Environ$", chroma.Keyword},
		{"End Sub", chroma.CommentMultiline}, {"ReM line comment", chroma.CommentSingle},
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

func TestFreeBasicFoldsMatchedDeclarationsOutsideComments(t *testing.T) {
	const source = "Declare Sub forward()\r\nType Record\n  Union\n    field As Integer\n  End\tUnion\nEnd Type\n" +
		"Sub Demo()\n  /' comment\nEnd Sub\n'/\n  print \"End Sub\"\nEnd Sub\nSub Incomplete()\n"
	want := []FoldRegion{{Header: 1, LastLine: 5, Level: 1}, {Header: 2, LastLine: 4, Level: 2}, {Header: 6, LastLine: 11, Level: 1}}
	for _, ext := range []string{".bas", ".BI"} {
		if got := FoldRegions(ext, source); !slices.Equal(got, want) {
			t.Errorf("%s folds = %+v, want %+v", ext, got, want)
		}
	}
}
