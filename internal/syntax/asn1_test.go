package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestASN1LexerPreservesMIBSyntaxAndByteRanges(t *testing.T) {
	lexer := lexers.Get("ASN.1")
	if lexer == nil {
		t.Fatal("ASN.1 lexer is missing")
	}
	const source = "DEMO DEFINITIONS ::= BEGIN\r\n" +
		"reading OBJECT-TYPE\r\n  SYNTAX Integer32 (0..100)\r\n" +
		"  MAX-ACCESS read-only\r\n  DESCRIPTION \"caf\u00e9\r\n    BOOLEAN -- still quoted\"\r\n" +
		"  ::=\r\n  { type2(7) 23 }\r\nflag BOOLEAN -- OBJECT is a comment\r\nEND"
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
		t.Fatal("tokenisation modified source bytes")
	}
	for _, tc := range []struct {
		value string
		style chroma.TokenType
	}{
		{"OBJECT-TYPE", chroma.Keyword},
		{"Integer32", chroma.KeywordType},
		{"read-only", chroma.KeywordConstant},
		{"::=", chroma.Operator},
		{"100", chroma.LiteralNumberInteger},
		{"caf\u00e9", chroma.LiteralStringDouble},
		{"BOOLEAN -- still quoted", chroma.LiteralStringDouble},
		{"type2", chroma.Text},
		{"23", chroma.LiteralNumber},
		{"-- OBJECT is a comment", chroma.CommentSingle},
		{"END", chroma.Keyword},
	} {
		start := strings.Index(source, tc.value)
		for _, kind := range styles[start : start+len(tc.value)] {
			if kind != tc.style {
				t.Errorf("%q = %v, want %v", tc.value, kind, tc.style)
				break
			}
		}
	}
	endBoolean := strings.LastIndex(source, "BOOLEAN")
	if got := styles[endBoolean]; got != chroma.NameClass {
		t.Errorf("unquoted BOOLEAN = %v, want descriptor", got)
	}
}
