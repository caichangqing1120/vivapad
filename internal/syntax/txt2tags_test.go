package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestTxt2tagsMarkupAndLiteralBlocks(t *testing.T) {
	const source = "= Overview =\r\n++ Numbered ++\r- **bold** //italic// __under__ --strike-- ``code``\n" +
		"+ second\n: term\n| cell | other |\n[name https://example.test] [photo.png]\n" +
		"% comment\r\n%!preproc: value\n%!postproc: value\n%!target: html\n\\**escaped**\n" +
		"```\n= inside verbatim =\n```\r\n%%%\n% inside comment\n%%%\n" +
		"\"\"\"\n**inside raw**\n\"\"\"\n'''\n//inside tagged//\n'''\n= Restored ="
	lexer := lexers.Get("txt2tags")
	if lexer == nil || lexer.Config().Name != "txt2tags" {
		t.Fatal("txt2tags lexer is missing")
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
		value string
		kind  chroma.TokenType
	}{
		{"= Overview =", chroma.GenericHeading}, {"++ Numbered ++", chroma.GenericHeading},
		{"- ", chroma.NameTag}, {": term", chroma.NameTag}, {"| cell | other |", chroma.NameTag},
		{"**bold**", chroma.GenericStrong}, {"//italic//", chroma.GenericEmph}, {"__under__", chroma.GenericUnderline},
		{"--strike--", chroma.GenericDeleted}, {"``code``", chroma.LiteralStringBacktick},
		{"[name https://example.test]", chroma.NameAttribute}, {"[photo.png]", chroma.NameAttribute},
		{"% comment", chroma.CommentSingle}, {"%!preproc: value", chroma.CommentPreproc},
		{"%!postproc: value", chroma.CommentPreproc}, {"%!target: html", chroma.KeywordNamespace},
		{"\\**escaped**", chroma.Text}, {"= inside verbatim =", chroma.LiteralStringBacktick},
		{"% inside comment", chroma.CommentMultiline}, {"**inside raw**", chroma.LiteralStringBacktick},
		{"//inside tagged//", chroma.LiteralStringBacktick}, {"= Restored =", chroma.GenericHeading},
	} {
		start := strings.Index(source, tc.value)
		for _, kind := range styles[start : start+len(tc.value)] {
			if kind != tc.kind {
				t.Errorf("%q classified as %v, want %v", tc.value, kind, tc.kind)
				break
			}
		}
	}
	ordered := strings.Index(source, "+ second")
	if styles[ordered] != chroma.NameTag || styles[ordered+2] != chroma.Text {
		t.Fatal("ordered list marker and list body are not distinguished")
	}
}

func TestTxt2tagsFenceEditRepaintsLaterLines(t *testing.T) {
	const heading = "= Context ="
	const original = "```\n" + heading + "\n```\n"
	h := NewTxt2tagsHighlighter()
	for _, tc := range []struct {
		source string
		kind   chroma.TokenType
	}{
		{original, chroma.LiteralStringBacktick},
		{"``\n" + heading + "\n```\n", chroma.GenericHeading},
		{original, chroma.LiteralStringBacktick},
	} {
		runs := h.ForText(tc.source)(".t2t", 1, heading, nil)
		color, _ := txt2tagsTokenColor(tc.kind)
		if len(runs) != 1 || runs[0].Start != 0 || runs[0].End != len(heading) || runs[0].Color != color {
			t.Fatalf("heading inside %q: runs = %v, want full line %v", tc.source, runs, tc.kind)
		}
	}
}
