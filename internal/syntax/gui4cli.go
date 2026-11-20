package syntax

import (
	_ "embed"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
)

// The five Gui4Cli command groups in Notepad++ v8.9.8.1 langs.model.xml.
//
//go:embed gui4cli_words.txt
var gui4CliWordData string

var gui4CliWords = func() map[string]chroma.TokenType {
	kinds := [...]chroma.TokenType{chroma.KeywordNamespace, chroma.NameFunctionMagic, chroma.NameAttribute, chroma.Keyword, chroma.NameBuiltin}
	words := make(map[string]chroma.TokenType)
	data := strings.ReplaceAll(strings.TrimSpace(gui4CliWordData), "\r\n", "\n")
	groups := strings.SplitN(data, "\n\n", len(kinds))
	// Events are looked up last even though they are the second word group.
	for _, i := range [...]int{0, 2, 3, 4, 1} {
		if i >= len(groups) {
			continue
		}
		for _, word := range strings.Fields(groups[i]) {
			word = strings.ToUpper(word)
			if _, exists := words[word]; !exists {
				words[word] = kinds[i]
			}
		}
	}
	return words
}()

type gui4CliLexer struct{ config chroma.Config }

var _ = lexers.Register(&gui4CliLexer{config: chroma.Config{Name: "Gui4Cli"}})

func (l *gui4CliLexer) Config() *chroma.Config                         { return &l.config }
func (l *gui4CliLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *gui4CliLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*gui4CliLexer) AnalyseText(string) float32                       { return 0 }

func (*gui4CliLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanGui4CliLine), nil
}

type Gui4CliHighlighter struct{ *lineHighlighter[bool] }

func NewGui4CliHighlighter() *Gui4CliHighlighter {
	h := newLineHighlighter(scanGui4CliLine)
	h.color = gui4CliTokenColor
	return &Gui4CliHighlighter{h}
}

func gui4CliTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.NameAttribute, chroma.NameFunctionMagic, chroma.NameBuiltin:
		return gfx.RGB(113, 76, 144), true
	default:
		return tokenColor(kind)
	}
}

func scanGui4CliLine(line string, inComment bool, emit func(int, int, chroma.TokenType)) bool {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	first := true
	for i := 0; i < len(line); {
		start := i
		if inComment || strings.HasPrefix(line[i:], "/*") {
			if !inComment {
				i += 2
			}
			end := strings.Index(line[i:], "*/")
			if end < 0 {
				style(start, len(line), chroma.CommentMultiline)
				return true
			}
			i += end + 2
			style(start, i, chroma.CommentMultiline)
			inComment = false
			continue
		}
		if strings.HasPrefix(line[i:], "//") {
			style(i, len(line), chroma.CommentSingle)
			return false
		}

		ch := line[i]
		i++
		switch {
		case ch == ' ' || ch == '\t':
			continue
		case ch == ';':
			style(start, i, chroma.Operator)
			first = true
			continue
		case ch == '\'' || ch == '"':
			kind := chroma.LiteralStringDouble
			if ch == '\'' {
				kind = chroma.LiteralStringSingle
			}
			for i < len(line) {
				c := line[i]
				i++
				if c == '\\' && i < len(line) {
					_, size := utf8.DecodeRuneInString(line[i:])
					i += size
				} else if c == ch {
					break
				}
			}
			style(start, i, kind)
		case ch == '\\':
			if i < len(line) {
				_, size := utf8.DecodeRuneInString(line[i:])
				i += size
			}
			style(start, i, chroma.LiteralStringEscape)
		case gui4CliWordByte(ch):
			for i < len(line) && gui4CliWordByte(line[i]) {
				i++
			}
			if first && emit != nil {
				if kind, ok := gui4CliWords[strings.ToUpper(line[start:i])]; ok {
					style(start, i, kind)
				}
			}
		case strings.IndexByte("*/-+()=%[]<>,:!&|$", ch) >= 0:
			style(start, i, chroma.Operator)
		}
		first = false
	}
	return inComment
}

func gui4CliWordByte(ch byte) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '.' || ch == '_' || ch == '#'
}

func gui4CliFoldRegions(value string) []FoldRegion {
	var regions []FoldRegion
	header, row := -1, 0
	inComment := false
	finish := func() {
		if header >= 0 && header < row-1 {
			regions = append(regions, FoldRegion{Header: header, LastLine: row - 1, Level: 1})
		}
	}
	for value != "" {
		end, next := syntaxLineBounds(value)
		inComment = scanGui4CliLine(value[:end], inComment, func(_, _ int, kind chroma.TokenType) {
			if (kind == chroma.KeywordNamespace || kind == chroma.NameFunctionMagic) && header != row {
				finish()
				header = row
			}
		})
		row++
		value = value[next:]
	}
	finish()
	return regions
}
