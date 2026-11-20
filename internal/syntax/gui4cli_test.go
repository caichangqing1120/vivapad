package syntax

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestGui4CliCommandPositionsAndContext(t *testing.T) {
	const source = " #include \"file.gui\"\r\nwindow demo\nxonload\r" +
		"  Attr ID ; If ready ; Say \"caf\u00e9; WINDOW\" ; Return\nSay WINDOW \\; XONOPEN // Say\n" +
		"/* outer\nXONCLOSE; WINDOW\n*/\nSay 'it\\'s; IF'\nSCI.XONSCI\n"
	lexer := lexers.Get("Gui4Cli")
	if lexer == nil || lexer.Config().Name != "Gui4Cli" {
		t.Fatal("Gui4Cli lexer is missing")
	}
	next, err := lexer.Tokenise(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	var decoded strings.Builder
	styles := make([]chroma.TokenType, 0, len(source))
	for tok := next(); tok != chroma.EOF; tok = next() {
		decoded.WriteString(tok.Value)
		for range len(tok.Value) {
			styles = append(styles, tok.Type)
		}
	}
	if decoded.String() != source {
		t.Fatal("tokenisation modified source bytes")
	}
	for _, tc := range []struct {
		word string
		kind chroma.TokenType
	}{
		{"#include", chroma.KeywordNamespace}, {"window", chroma.KeywordNamespace}, {"xonload", chroma.NameFunctionMagic},
		{"Attr", chroma.NameAttribute}, {"ID", chroma.Text}, {"If", chroma.Keyword}, {"Say", chroma.NameBuiltin},
		{"\"caf\u00e9; WINDOW\"", chroma.LiteralStringDouble}, {"Return", chroma.Keyword}, {"XONOPEN", chroma.Text},
		{"// Say", chroma.CommentSingle}, {"XONCLOSE; WINDOW", chroma.CommentMultiline}, {"'it\\'s; IF'", chroma.LiteralStringSingle},
		{"SCI.XONSCI", chroma.NameFunctionMagic},
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

func TestGui4CliFoldsOnlyLexicalGlobalAndEventHeaders(t *testing.T) {
	const source = "Window one\r\nSay 'XONOPEN'\n/*\rXONLOAD\n*/\nXONLOAD\nSay two\nSay three ; XONCLOSE\nSay four\nWindow empty\n"
	want := []FoldRegion{{Header: 0, LastLine: 4, Level: 1}, {Header: 5, LastLine: 6, Level: 1}, {Header: 7, LastLine: 8, Level: 1}}
	if got := FoldRegions(".GUI", source); !slices.Equal(got, want) {
		t.Errorf("Gui4Cli folds = %+v, want %+v", got, want)
	}
}
