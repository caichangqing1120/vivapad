package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestKixTokenKindsAndMultilineState(t *testing.T) {
	const source = "DiM $caf\u00e9 = AbS(.125) + &7 ; comment\r\n@CrLf @not_a_macro\r\n" +
		"/* block\rcontinued */ ReTuRn 2\n\"single starter\r\n/* not ; a comment\"\n'end\nquote'"
	lexer := lexers.Get("KIXtart")
	if lexer == nil || lexer.Config().Name != "KIXtart" {
		t.Fatal("KIXtart lexer is missing")
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
		text string
		kind chroma.TokenType
	}{
		{"DiM", chroma.Keyword}, {"$caf\u00e9", chroma.NameVariable},
		{"AbS", chroma.NameFunction}, {".125", chroma.LiteralNumber}, {"&7", chroma.LiteralNumber},
		{"; comment", chroma.CommentSingle}, {"@CrLf", chroma.NameConstant}, {"@not_a_macro", chroma.Text},
		{"/* block", chroma.CommentMultiline}, {"continued */", chroma.CommentMultiline}, {"ReTuRn", chroma.Keyword},
		{"\"single starter", chroma.LiteralStringDouble}, {"/* not ; a comment\"", chroma.LiteralStringDouble},
		{"'end", chroma.LiteralStringSingle}, {"quote'", chroma.LiteralStringSingle},
	} {
		start := strings.Index(source, tc.text)
		for _, kind := range styles[start : start+len(tc.text)] {
			if kind != tc.kind {
				t.Errorf("%q classified as %v, want %v", tc.text, kind, tc.kind)
				break
			}
		}
	}
}
