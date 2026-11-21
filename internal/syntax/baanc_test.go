package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestBaanCReferenceTokenBoundaries(t *testing.T) {
	lexer := lexers.Get("BaanC")
	if lexer == nil || lexer.Config().Name != "BaanC" {
		t.Fatal("BaanC lexer is missing")
	}
	const source = "DoMaIn ttdate\r\nFUNCTION long demo()\n" +
		"AFTER.CHOICE: before.custom: after.custom\r" +
		"abort.io rec.is.mandatory() ignored.is.mandatory zoomreturn$ ttccom001 tccom001.name\n" +
		"\"caf\u00e9 | still a string\" | SELECT\r\n" +
		"functionusage\r\nselect | still documentation\n\u0130 ENDFUNCTIONUSAGE select\r\n" +
		" #define FOO 12\n^continued\r\n #unknown FOO\n SELECT -1.25e-3 +0xAf 7suffix\n" +
		"tccom001._index12 tccom001._indexinvalid rcd.ttccom001 tccom.dll1234.Action"
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	var joined strings.Builder
	type token struct {
		kind chroma.TokenType
		text string
	}
	got := make(map[token]int)
	for item := iterator(); item != chroma.EOF; item = iterator() {
		joined.WriteString(item.Value)
		got[token{item.Type, item.Value}]++
	}
	if joined.String() != source {
		t.Fatal("lexer changed line endings or source bytes")
	}
	for _, want := range []token{
		{chroma.Keyword, "DoMaIn"}, {chroma.NameClass, "ttdate"},
		{chroma.NameFunction, "demo"}, {chroma.NameLabel, "AFTER.CHOICE"},
		{chroma.KeywordNamespace, "before.custom"},
		{chroma.NameBuiltin, "abort.io"}, {chroma.NameBuiltin, "rec.is.mandatory"},
		{chroma.NameVariable, "zoomreturn$"}, {chroma.NameClass, "ttccom001"},
		{chroma.NameAttribute, "tccom001.name"},
		{chroma.LiteralStringDouble, "\"caf\u00e9 | still a string\""},
		{chroma.CommentSingle, "| SELECT"},
		{chroma.CommentMultiline, "select | still documentation"},
		{chroma.CommentMultiline, "\u0130 ENDFUNCTIONUSAGE"}, {chroma.Keyword, "select"},
		{chroma.CommentPreproc, "#define FOO 12"}, {chroma.CommentPreproc, "^continued"},
		{chroma.LiteralNumber, "-1.25e-3"}, {chroma.LiteralNumber, "+0xAf"},
		{chroma.NameClass, "tccom001._index12"}, {chroma.NameClass, "rcd.ttccom001"},
		{chroma.NameFunction, "tccom.dll1234.Action"},
	} {
		if got[want] != 1 {
			t.Errorf("%q: got %d tokens of type %s, want one", want.text, got[want], want.kind)
		}
	}
	for _, unwanted := range []token{{chroma.NameBuiltin, "ignored.is.mandatory"}, {chroma.NameLabel, "after.custom"}, {chroma.LiteralNumber, "7"}, {chroma.CommentPreproc, "#unknown FOO"}, {chroma.NameClass, "tccom001._indexinvalid"}} {
		if got[unwanted] != 0 {
			t.Errorf("unexpected %s token %q", unwanted.kind, unwanted.text)
		}
	}
}
