package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestServerPageTokensKeepMarkupAndScriptContexts(t *testing.T) {
	type token struct {
		text string
		kind chroma.TokenType
	}
	for _, tc := range []struct {
		name, input string
		want        []token
	}{
		{"ASP", "<p>caf\u00e9</p>\r\n<%@ Page Title=\"not a language='C#' directive %>\" %>\n<%\rDim value = 42\n' a comment\r\n%>\n" +
			"<%@ Page\nLanguage=\"C#\" %>\n<% if (true) { } %><strong>back</strong>", []token{
			{"p", chroma.NameTag}, {"Dim", chroma.Keyword}, {"42", chroma.LiteralNumber},
			{"' a comment", chroma.CommentSingle}, {" Page Title=\"not a language='C#' directive %>\" %>", chroma.CommentPreproc},
			{"Language=\"C#\" %>", chroma.CommentPreproc},
			{"if", chroma.Keyword}, {"strong", chroma.NameTag},
		}},
		{"JSP", "<!-- <% if %> -->\r\n<%-- hidden\n<%= 123 %>\n--%>\r" +
			"<span title=\"pre <%= \"x %> y\" %> post\">\n<% /* still\nnot %> code */ return 42; %></span>", []token{
			{"<!-- <% if %> -->", chroma.CommentMultiline}, {"<%= 123 %>", chroma.CommentMultiline},
			{"span", chroma.NameTag}, {"title", chroma.NameAttribute},
			{"\"x %> y\"", chroma.LiteralString}, {" post\"", chroma.LiteralString},
			{"not %> code */", chroma.CommentMultiline}, {"return", chroma.Keyword}, {"42", chroma.LiteralNumber},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lexer := lexers.Get(tc.name)
			if lexer == nil || lexer.Config().Name != tc.name {
				t.Fatalf("%s lexer is missing", tc.name)
			}
			iterator, err := lexer.Tokenise(nil, tc.input)
			if err != nil {
				t.Fatal(err)
			}
			var joined strings.Builder
			var got []token
			for item := iterator(); item != chroma.EOF; item = iterator() {
				joined.WriteString(item.Value)
				got = append(got, token{item.Value, item.Type})
			}
			if joined.String() != tc.input {
				t.Fatalf("the lexer changed input bytes: %q", joined.String())
			}
			for _, want := range tc.want {
				found := false
				for _, item := range got {
					if item.text == want.text && (item.kind == want.kind || item.kind.InSubCategory(want.kind) && (want.kind == chroma.LiteralString || want.kind == chroma.LiteralNumber || want.kind == chroma.Keyword)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing %v; got %v", want, got)
				}
			}
		})
	}
}
