package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestNncrontabTokens(t *testing.T) {
	const source = "#( backup\nTime: 30 * * * *\nAction: BEEP NoLog Time-extra time\r" +
		"FILE-EXIST \"caf\u00e9 %TEMP% \\\"#(\" <%VALUE>\n<JScript> Day@\n" +
		"\\ action BEEP\n# comment NoLog\n)# end\n\"unfinished\nIF\n%unclosed\nNoActive"
	lexer := lexers.Get("Nncrontab")
	if lexer == nil || lexer.Config().Name != "Nncrontab" {
		t.Fatal("Nncrontab lexer is missing")
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
		{"#( backup", chroma.NameTag}, {"Time", chroma.Keyword}, {"30", chroma.LiteralNumberInteger}, {"*", chroma.Operator},
		{"Action", chroma.Keyword}, {"BEEP", chroma.NameBuiltin}, {"NoLog", chroma.KeywordPseudo},
		{"Time-extra", chroma.Text}, {"time", chroma.Text}, {"FILE-EXIST", chroma.NameBuiltin},
		{"caf\u00e9 ", chroma.LiteralStringDouble}, {"%TEMP%", chroma.LiteralStringInterpol}, {"\\\"#(\"", chroma.LiteralStringDouble},
		{"<%VALUE>", chroma.LiteralStringInterpol}, {"<JScript>", chroma.NameBuiltin}, {"Day@", chroma.NameBuiltin},
		{"\\ action BEEP", chroma.CommentSingle}, {"# comment NoLog", chroma.CommentSingle}, {")# end", chroma.NameTag},
		{"\"unfinished", chroma.LiteralStringDouble}, {"IF", chroma.Keyword}, {"%unclosed", chroma.LiteralStringInterpol}, {"NoActive", chroma.KeywordPseudo},
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

func TestNncrontabEditorHighlightsScheduleAndSubstitutions(t *testing.T) {
	var h Highlighter
	if runs := h.Highlight(".spf", `Time: 30 * %TEMP%`, nil); len(runs) != 4 {
		t.Fatalf("schedule render runs = %v; want a section, number, wildcard and substitution", runs)
	}
}
