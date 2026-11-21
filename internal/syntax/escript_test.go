package syntax

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestESCRIPTLexerPreservesBytesAndLineContext(t *testing.T) {
	const source = "PROGRAM demo\r\n  VaR X := .5\r  BaSiC();\n  \"text \\" +
		"\" quote\r\nENDPROGRAM\"\n/* BEGIN\rENDWHILE */\r\n// comment\\\r\ncontinuation\nENDPROGRAM"
	lexer := lexers.Get("ESCRIPT")
	if lexer == nil || lexer.Config().Name != "ESCRIPT" {
		t.Fatal("ESCRIPT lexer is missing")
	}
	iter, err := lexer.Tokenise(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	var original strings.Builder
	var styles []chroma.TokenType
	for token := iter(); token != chroma.EOF; token = iter() {
		original.WriteString(token.Value)
		for range len(token.Value) {
			styles = append(styles, token.Type)
		}
	}
	if original.String() != source {
		t.Fatal("ESCRIPT tokens changed the original bytes")
	}
	for _, tc := range []struct {
		part string
		kind chroma.TokenType
	}{
		{"PROGRAM", chroma.KeywordDeclaration}, {"VaR", chroma.Keyword}, {"BaSiC", chroma.NameBuiltin},
		{".5", chroma.LiteralNumber}, {"quote", chroma.LiteralStringDouble},
		{"ENDPROGRAM\"", chroma.LiteralStringDouble}, {"ENDWHILE */", chroma.CommentMultiline},
		{"continuation", chroma.CommentSingle},
	} {
		start := strings.Index(source, tc.part)
		for _, style := range styles[start : start+len(tc.part)] {
			if style != tc.kind {
				t.Errorf("%q: got %s, want %s", tc.part, style, tc.kind)
				break
			}
		}
	}
}

func TestESCRIPTFoldsIgnoreFakeEndingsAndTreatElseIfAsAContinuation(t *testing.T) {
	const source = "Program demo\r\nFunction work()\rIf ready\n\"endif\"\nElse If pending\n/* endfunction\r\nendif */\r\nElseif retry\nForeach item\nreturn\nEndforeach\nEndif\nEndfunction\nEndprogram"
	want := []FoldRegion{{Header: 0, LastLine: 13, Level: 1}, {Header: 1, LastLine: 12, Level: 2},
		{Header: 2, LastLine: 11, Level: 3}, {Header: 8, LastLine: 10, Level: 4}}
	for _, mode := range []string{".SRC", ".em"} {
		if got := FoldRegions(mode, source); !slices.Equal(got, want) {
			t.Fatalf("folds for %s = %+v, want %+v", mode, got, want)
		}
	}
}
