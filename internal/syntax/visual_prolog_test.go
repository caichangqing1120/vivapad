package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestVisualPrologOriginalExtensionMappings(t *testing.T) {
	for _, extension := range []string{".PRO", ".cl", ".i", ".pack", ".ph"} {
		language, ok := LanguageByMode(extension)
		if !ok || language.SourceID != "IDM_LANG_VISUALPROLOG" || language.NPPName != "visualprolog" || language.LineComment != "%" {
			t.Fatalf("mode %s = %+v", extension, language)
		}
	}
}

func TestVisualPrologTokensKeepStateAndMixedEOL(t *testing.T) {
	const source = "#include \"core\\u6587\"\r\nclass demo\r\n    if \u0394elta = 0x2a then _Tmp\n" +
		"% @short details\r/* outer /* inner */\r\nstill outer */ end class\n" +
		"@<verbatim >>\r\ncontinuation> [|literal {| Value |}:expr tail\r\nmore |] goal"
	lexer := lexers.Get("Visual Prolog")
	if lexer == nil || lexer.Config().Name != "Visual Prolog" {
		t.Fatal("Visual Prolog lexer is missing")
	}
	iter, err := lexer.Tokenise(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	var original strings.Builder
	var kinds []chroma.TokenType
	for token := iter(); token != chroma.EOF; token = iter() {
		original.WriteString(token.Value)
		for range len(token.Value) {
			kinds = append(kinds, token.Type)
		}
	}
	if original.String() != source {
		t.Fatal("lexing changed source bytes")
	}
	for _, tc := range []struct {
		word string
		kind chroma.TokenType
	}{
		{"#include", chroma.CommentPreproc}, {`\u6587`, chroma.LiteralStringEscape},
		{"class", chroma.KeywordDeclaration}, {"if", chroma.Keyword}, {"\u0394elta", chroma.NameVariable},
		{"0x2a", chroma.LiteralNumberHex}, {"_Tmp", chroma.NameVariableMagic}, {"@short", chroma.CommentSpecial},
		{"still outer */", chroma.CommentMultiline}, {"end", chroma.KeywordDeclaration},
		{"continuation>", chroma.LiteralString}, {"[|literal ", chroma.LiteralStringOther},
		{"Value", chroma.NameVariable}, {"|}:expr", chroma.StringInterpol}, {"more |]", chroma.LiteralStringOther},
		{"goal", chroma.KeywordDeclaration},
	} {
		start := strings.Index(source, tc.word)
		for _, kind := range kinds[start : start+len(tc.word)] {
			if kind != tc.kind {
				t.Errorf("%q: got %s, want %s", tc.word, kind, tc.kind)
				break
			}
		}
	}
}
