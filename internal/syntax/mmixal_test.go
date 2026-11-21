package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestMMIXALLexerKeepsColumnRolesAndSourceBytes(t *testing.T) {
	const source = "@i sample.mms\r\n" +
		"Main\tGET $1,:rA note\r\n" +
		"\t2ADDU $2,$3,400\r\n" +
		"\tLDA $1,1F\r\n" +
		"\tLOC Text_Segment+#ff\r\n" +
		"\tBYTE \"caf\u00e9 note\",'x' words\r\n" +
		"\tSET $4,9%2\r\n" +
		"  % full comment\r\n" +
		"\tadd $0\r\n" +
		"\tBYTE \"unterminated\r\n" +
		"\tTRAP 0,Fputs,StdOut\r\n" +
		"\tSWYM"
	lexer := lexers.Get("MMIXAL")
	if lexer == nil || lexer.Config().Name != "MMIXAL" {
		t.Fatal("MMIXAL lexer is missing")
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
		t.Fatal("tokenisation changed source bytes")
	}
	for _, tc := range []struct {
		value string
		kind  chroma.TokenType
	}{
		{"@i sample.mms", chroma.CommentPreproc}, {"Main", chroma.NameLabel},
		{"GET", chroma.Keyword}, {"$1", chroma.NameVariable}, {":rA", chroma.NameVariable},
		{" note", chroma.CommentSingle}, {"2ADDU", chroma.Keyword}, {"400", chroma.LiteralNumberInteger},
		{"1F", chroma.Text}, {"Text_Segment", chroma.NameConstant}, {"#ff", chroma.LiteralNumberHex},
		{"\"caf\u00e9 note\"", chroma.LiteralStringDouble}, {"'x'", chroma.LiteralStringChar},
		{"%", chroma.Operator}, {"% full comment", chroma.CommentSingle}, {"add", chroma.Error},
		{"\"unterminated", chroma.LiteralStringDouble}, {"TRAP", chroma.Keyword}, {"Fputs", chroma.NameConstant}, {"SWYM", chroma.Keyword},
	} {
		start := strings.Index(source, tc.value)
		for i, kind := range styles[start : start+len(tc.value)] {
			if kind != tc.kind {
				t.Errorf("%q byte %d = %v, want %v", tc.value, i, kind, tc.kind)
				break
			}
		}
	}
}
