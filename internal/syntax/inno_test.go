package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestInnoSetupLexerPreservesBytesAndSectionContext(t *testing.T) {
	fixtures := []struct {
		line, token string
		kind        chroma.TokenType
	}{
		{"  [sEtUp]", "[sEtUp]", chroma.NameTag},
		{"AppName = \"Demo\"", "AppName", chroma.Keyword},
		{"AppName without assignment", "AppName", chroma.Text},
		{"  ; AppName = ignored", "AppName", chroma.CommentSingle},
		{"  #DeFiNe AppTitle \"Demo\"", "#DeFiNe", chroma.CommentPreproc},
		{"[Files]", "[Files]", chroma.NameTag},
		{"Source: \"data.txt\"; DestDir: {app}", "DestDir", chroma.Keyword},
		{"Unrecognized: {{app}", "{{app}", chroma.Text},
		{"DestDir: {app}", "{app}", chroma.LiteralStringInterpol},
		{"[CoDe]", "[CoDe]", chroma.NameTag},
		{"{ opening", "opening", chroma.CommentMultiline},
		{"[Setup]", "[Setup]", chroma.CommentMultiline},
		{"close } procedure Run;", "procedure", chroma.Keyword},
		{"(* second", "second", chroma.CommentMultiline},
		{"close *) BEGIN", "BEGIN", chroma.Keyword},
		{"  // end;", "end", chroma.CommentSingle},
		{"[Unknown]", "[Unknown]", chroma.Text},
		{"end;", "end", chroma.Keyword},
		{"[Messages]", "[Messages]", chroma.NameTag},
		{"AppName=\"message\"", "\"message\"", chroma.Text},
		{"[CustomMessages]", "[CustomMessages]", chroma.NameTag},
		{"AppName=\"custom\"", "\"custom\"", chroma.Text},
		{"[Setup]", "[Setup]", chroma.NameTag},
		{"AppName=\"caf\u00e9\"", "\"caf\u00e9\"", chroma.LiteralStringDouble},
	}
	lexer := lexers.Get("Inno Setup")
	if lexer == nil {
		t.Fatal("Inno Setup lexer is missing")
	}
	var source strings.Builder
	type expectation struct {
		start, end int
		kind       chroma.TokenType
	}
	var expected []expectation
	for _, fixture := range fixtures {
		start := source.Len() + strings.Index(fixture.line, fixture.token)
		expected = append(expected, expectation{start, start + len(fixture.token), fixture.kind})
		source.WriteString(fixture.line + "\r\n")
	}
	next, err := lexer.Tokenise(nil, source.String())
	if err != nil {
		t.Fatal(err)
	}
	var decoded strings.Builder
	styles := make([]chroma.TokenType, 0, source.Len())
	for token := next(); token != chroma.EOF; token = next() {
		decoded.WriteString(token.Value)
		for range len(token.Value) {
			styles = append(styles, token.Type)
		}
	}
	if decoded.String() != source.String() {
		t.Fatal("the lexer lost source bytes")
	}
	for _, match := range expected {
		for _, style := range styles[match.start:match.end] {
			if style != match.kind {
				t.Errorf("%q at %d is %v, want %v", source.String()[match.start:match.end], match.start, style, match.kind)
				break
			}
		}
	}
}
