package syntax

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestPureBasicTokensKeepSourceBytes(t *testing.T) {
	const source = "entry: PrOcEdUrE Demo()\r\nCompilerIf #PB_Compiler_OS = #PB_OS_Windows\n" +
		"  value = $2A + %101 + .5e-2\rDebug \"caf\u00e9; not a comment\" ; actual comment\n" +
		"CompilerEndIf\n/' shared comment\r\nEndProcedure\n'/\nEndProcedure\n"
	lexer := lexers.Get("PureBasic")
	if lexer == nil || lexer.Config().Name != "PureBasic" {
		t.Fatal("PureBasic lexer is missing")
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
		{"entry:", chroma.NameLabel}, {"PrOcEdUrE", chroma.Keyword}, {"CompilerIf", chroma.CommentPreproc},
		{"#PB_Compiler_OS", chroma.NameConstant}, {"$2A", chroma.LiteralNumberHex}, {"%101", chroma.LiteralNumberBin},
		{".5e-2", chroma.LiteralNumber}, {"\"caf\u00e9; not a comment\"", chroma.LiteralStringDouble},
		{"; actual comment", chroma.CommentSingle}, {"EndProcedure", chroma.CommentMultiline},
		{"CompilerEndIf", chroma.CommentPreproc},
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

func TestPureBasicFoldsMatchedDeclarationsOutsideComments(t *testing.T) {
	const source = "Declare Demo()\r\nProcedure Demo()\n/' hidden\nEndProcedure\n'/\nDebug \"EndProcedure\"\nEndProcedure\n" +
		"; Structure Fake\nStructure Record\n  count.i\nEndStructure\nInterface Incomplete\n"
	want := []FoldRegion{{Header: 1, LastLine: 6, Level: 1}, {Header: 8, LastLine: 10, Level: 1}}
	if got := FoldRegions(".PB", source); !slices.Equal(got, want) {
		t.Errorf("PureBasic folds = %+v, want %+v", got, want)
	}
}
