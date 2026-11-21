package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestSpiceClassifiesNetlistTokensWithoutChangingBytes(t *testing.T) {
	const source = "* Example circuit\r\n.TRAN 100n 1.5e-3\n.param gain=SQRT(2..4)\r" +
		"R1 in out 4.7k*8 *~ r\u00e9sistance\r\n  * 12\n#ff\r\n.MODEL sample D"
	lexer := lexers.Get("Spice")
	if lexer == nil || lexer.Config().Name != "Spice" {
		t.Fatal("Spice lexer is missing")
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
		{"* Example circuit", chroma.CommentSingle}, {"TRAN", chroma.Keyword},
		{"100n", chroma.LiteralNumber}, {"1.5e-3", chroma.LiteralNumber},
		{"param", chroma.KeywordDeclaration}, {"SQRT", chroma.NameFunction},
		{"..", chroma.Punctuation}, {"4.7k", chroma.LiteralNumber},
		{"*~ r\u00e9sistance", chroma.CommentSingle}, {"12", chroma.LiteralNumber},
		{"#ff", chroma.LiteralNumber}, {"MODEL", chroma.KeywordDeclaration},
		{"R1", chroma.Text}, {"gain", chroma.Text},
	} {
		start := strings.Index(source, tc.text)
		for _, kind := range styles[start : start+len(tc.text)] {
			if kind != tc.kind {
				t.Errorf("%q classified as %v, want %v", tc.text, kind, tc.kind)
				break
			}
		}
	}
	for _, field := range []string{"4.7k*8", "  * 12"} {
		star := strings.Index(source, field) + strings.IndexByte(field, '*')
		if got := styles[star]; got != chroma.Punctuation {
			t.Errorf("asterisk in %q classified as %v, want punctuation", field, got)
		}
	}
	for _, offset := range []int{strings.Index(source, "2..4"), strings.Index(source, "2..4") + 3} {
		if got := styles[offset]; got != chroma.LiteralNumber {
			t.Errorf("number at %d classified as %v", offset, got)
		}
	}
}
