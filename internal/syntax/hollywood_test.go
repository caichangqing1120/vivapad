package syntax

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestHollywoodTokensPreserveMixedEOL(t *testing.T) {
	const source = "@DISPLAY {Title=\"Demo\"}\r\nFUNCTION Demo()\rPrint(#RED, $2a, 0X1f, .5)\n" +
		"cairo.context Arc\n/* EndFunction\ncomment */\n[[Function\nlong text]] ; EndFunction\nEndFunction"
	lexer := lexers.Get("Hollywood")
	if lexer == nil || lexer.Config().Name != "Hollywood" {
		t.Fatal("Hollywood lexer is missing")
	}
	iter, err := lexer.Tokenise(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	var decoded strings.Builder
	var styles []chroma.TokenType
	for tok := iter(); tok != chroma.EOF; tok = iter() {
		decoded.WriteString(tok.Value)
		for range len(tok.Value) {
			styles = append(styles, tok.Type)
		}
	}
	if decoded.String() != source {
		t.Fatal("tokens do not preserve source bytes")
	}
	for _, tc := range []struct {
		word string
		kind chroma.TokenType
	}{
		{"@DISPLAY", chroma.CommentPreproc}, {"FUNCTION", chroma.Keyword}, {"Print", chroma.NameBuiltin},
		{"#RED", chroma.NameConstant}, {"$2a", chroma.LiteralNumberHex}, {"0X1f", chroma.LiteralNumberHex},
		{".5", chroma.LiteralNumber}, {"cairo.context", chroma.NameFunction}, {"Arc", chroma.NameFunctionMagic},
		{"/* EndFunction", chroma.CommentMultiline}, {"comment */", chroma.CommentMultiline},
		{"[[Function", chroma.LiteralString}, {"long text]]", chroma.LiteralString}, {"; EndFunction", chroma.CommentSingle},
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

func TestHollywoodFoldsNestedFunctionsOutsideLiterals(t *testing.T) {
	const value = "Function Outer()\r\n/* EndFunction\n*/\nLocal Function Nested()\nPrint([[\nEndFunction\n]])\nEndFunction\rEndFunction\nFunction Open()"
	want := []FoldRegion{{Header: 0, LastLine: 8, Level: 1}, {Header: 3, LastLine: 7, Level: 2}}
	if got := FoldRegions(".HWS", value); !slices.Equal(got, want) {
		t.Fatalf("Hollywood folds = %v, want %v", got, want)
	}
}
