package syntax

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestOScriptClassifiesMemberAndCommentContext(t *testing.T) {
	const source = "start: INTEGER x = -1.5e+2\r\nCacheTree.Size, CacheTree.Get()\nEcho($$state, True)\r" +
		"value = 'it''s' // note\n/* first\r\nend */ return\n#ifdef DOC\nfunction caf\u00e9\n#endif\n"
	lexer := lexers.Get("OScript")
	if lexer == nil || lexer.Config().Name != "OScript" {
		t.Fatal("OScript lexer is missing")
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
		{"start:", chroma.NameLabel}, {"INTEGER", chroma.KeywordType}, {"-1.5e+2", chroma.LiteralNumber},
		{"CacheTree", chroma.NameClass}, {"Size", chroma.NameProperty}, {"Get", chroma.NameFunctionMagic},
		{"Echo", chroma.NameFunction}, {"$$state", chroma.NameVariableGlobal}, {"True", chroma.KeywordConstant},
		{"'it''s'", chroma.LiteralStringSingle}, {"// note", chroma.CommentSingle}, {"end */", chroma.CommentMultiline},
		{"return", chroma.Keyword}, {"#ifdef DOC", chroma.CommentSpecial}, {"function caf\u00e9", chroma.CommentSpecial},
		{"#endif", chroma.CommentSpecial},
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

func TestOScriptFoldsMatchingStatementsAndConditionalComments(t *testing.T) {
	const source = "Integer function Demo()\r\n/* comment\r\nend\r\n*/\nIf True\n  Echo(\"end\")\nEnd\nEnd\n" +
		"#ifdef DOC\nfunction for while\n#endif\nRepeat\n  Echo(1)\nUntil True\nFunction Unmatched()\n"
	want := []FoldRegion{{Header: 0, LastLine: 7, Level: 1}, {Header: 1, LastLine: 3, Level: 2}, {Header: 4, LastLine: 6, Level: 2}, {Header: 8, LastLine: 10, Level: 1}, {Header: 11, LastLine: 13, Level: 1}}
	if got := FoldRegions(".OSX", source); !slices.Equal(got, want) {
		t.Errorf("OScript folds = %+v, want %+v", got, want)
	}
}
