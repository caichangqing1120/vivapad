package syntax

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestREBOLLexerRetainsMultilineSyntaxAndBytes(t *testing.T) {
	const source = "shell preface\r\nrebol [Title: \"Caf\u00e9\"]\r\nPrint/only -2.5\nINTEGER! integer? ; note\r" +
		"{first {nested\r\nsecond} third}\r\ncomment\n{ignored [\ncontinued]}\n16#{DEAD\r\nBEEF}\n" +
		"%file.txt https://example.test/a person@example.test #\"a\" <tag> #issue $12.00 USD$5.50 \"quoted^\"text\""
	lexer := lexers.Get("REBOL")
	if lexer == nil || lexer.Config().Name != "REBOL" {
		t.Fatal("REBOL lexer is missing")
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
		{"shell preface", chroma.CommentPreproc}, {"rebol", chroma.KeywordNamespace},
		{"\"Caf\u00e9\"", chroma.LiteralStringDouble}, {"Print/only", chroma.Keyword},
		{"-2.5", chroma.LiteralNumber}, {"INTEGER!", chroma.KeywordType}, {"integer?", chroma.NameFunction},
		{"; note", chroma.CommentSingle}, {"{first {nested", chroma.LiteralString},
		{"second} third}", chroma.LiteralString}, {"{ignored [", chroma.CommentMultiline},
		{"continued]}", chroma.CommentMultiline}, {"16#{DEAD", chroma.LiteralNumberHex}, {"BEEF}", chroma.LiteralNumberHex},
		{"%file.txt", chroma.LiteralStringOther}, {"https://example.test/a", chroma.LiteralStringOther},
		{"person@example.test", chroma.LiteralStringOther}, {"#\"a\"", chroma.LiteralStringChar},
		{"<tag>", chroma.NameTag}, {"#issue", chroma.NameConstant}, {"$12.00", chroma.LiteralNumber},
		{"USD$5.50", chroma.LiteralNumber},
		{"\"quoted^\"text\"", chroma.LiteralStringDouble},
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

func TestREBOLFoldsOnlyVisibleSyntaxBrackets(t *testing.T) {
	const source = "[preface\r\nREBOL []\r\nmain: [\r\n{literal [\r\n]}\r\ncomment { ignore ] }\r\n" +
		"16#{\r\n[AB]\r\n}\r\n; ]\r\nprint \"[\"\r\ninner: [\r\nprint 1\r\n]\r\n]\r\n"
	want := []FoldRegion{{Header: 2, LastLine: 14, Level: 1}, {Header: 11, LastLine: 13, Level: 2}}
	for _, ext := range []string{".reb", ".R2", ".r3"} {
		if got := FoldRegions(ext, source); !slices.Equal(got, want) {
			t.Errorf("%s folds = %+v, want %+v", ext, got, want)
		}
	}
}
