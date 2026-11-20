package syntax

import (
	_ "embed"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// The three nonempty REBOL keyword groups in Notepad++ v8.9.8.1.
//
//go:embed rebol_words.txt
var rebolWordData string

var rebolWords = func() map[string]chroma.TokenType {
	kinds := [...]chroma.TokenType{chroma.Keyword, chroma.NameFunction, chroma.KeywordType}
	words := make(map[string]chroma.TokenType)
	for i, group := range strings.Split(strings.TrimSpace(rebolWordData), "\n\n") {
		for _, word := range strings.Fields(group) {
			words[word] = kinds[i]
		}
	}
	return words
}()

var rebolHeader = regexp.MustCompile(`(?i)\brebol[ \t]*\[`)

type rebolState struct {
	started, commentPending bool
	depth                   int
	block                   chroma.TokenType
}

type rebolLexer struct{ config chroma.Config }

var _ = lexers.Register(&rebolLexer{config: chroma.Config{Name: "REBOL"}})

func (l *rebolLexer) Config() *chroma.Config                         { return &l.config }
func (l *rebolLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *rebolLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*rebolLexer) AnalyseText(string) float32                       { return 0 }

func (*rebolLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanREBOLLine), nil
}

type REBOLHighlighter struct{ *lineHighlighter[rebolState] }

func NewREBOLHighlighter() *REBOLHighlighter {
	return &REBOLHighlighter{newLineHighlighter(scanREBOLLine)}
}

func scanREBOLLine(line string, state rebolState, emit func(int, int, chroma.TokenType)) rebolState {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end && kind != chroma.Text {
			emit(start, end, kind)
		}
	}
	consumeBlock := func(start, next int) int {
		for next < len(line) && state.depth > 0 {
			ch := line[next]
			next++
			switch {
			case ch == '^' && state.block != chroma.LiteralNumberHex && next < len(line):
				next++
			case ch == '{' && state.block != chroma.LiteralNumberHex:
				state.depth++
			case ch == '}':
				state.depth--
			}
		}
		style(start, next, state.block)
		return next
	}
	i := 0
	if !state.started {
		header := rebolHeader.FindStringIndex(line)
		if header == nil {
			style(0, len(line), chroma.CommentPreproc)
			return state
		}
		style(0, header[0], chroma.CommentPreproc)
		i = header[0] + len("REBOL")
		style(header[0], i, chroma.KeywordNamespace)
		state.started = true
	} else if state.depth > 0 {
		i = consumeBlock(0, 0)
	}
	for i < len(line) {
		start := i
		ch := line[i]
		if ch == ' ' || ch == '\t' || ch == '\f' || ch == '\v' {
			i++
			continue
		}
		comment := state.commentPending
		state.commentPending = false
		i++
		switch {
		case ch == ';':
			style(start, len(line), chroma.CommentSingle)
			return state
		case ch == '{':
			state.depth, state.block = 1, chroma.LiteralString
			if comment {
				state.block = chroma.CommentMultiline
			}
			i = consumeBlock(start, i)
		case rebolBinaryOpen(line[start:]) != 0:
			state.depth, state.block = 1, chroma.LiteralNumberHex
			i = consumeBlock(start, start+rebolBinaryOpen(line[start:]))
		case ch == '"' || ch == '#' && i < len(line) && line[i] == '"':
			kind := chroma.LiteralStringDouble
			if ch == '#' {
				i++
				kind = chroma.LiteralStringChar
			}
			i = rebolQuotedEnd(line, i)
			style(start, i, kind)
		case strings.IndexByte("[]()}", ch) >= 0:
			style(start, i, chroma.Punctuation)
		case ch == '<' && i < len(line) && rebolTagStart(line[i]):
			i = rebolTagEnd(line, i)
			style(start, i, chroma.NameTag)
		default:
			for i < len(line) && strings.IndexByte(" \t\f\v[](){};\"", line[i]) < 0 {
				i++
			}
			word := line[start:i]
			kind, pending := rebolWordKind(word)
			state.commentPending = pending
			style(start, i, kind)
		}
	}
	return state
}

func rebolWordKind(word string) (chroma.TokenType, bool) {
	switch word {
	case "+", "-", "*", "**", "/", "//", "<", "<=", "<>", ">", ">=", "=", "==", "=?", "?", "??":
		return chroma.Operator, false
	}
	switch {
	case word[0] == '%' || strings.Contains(word, "://") || strings.IndexByte(word, '@') > 0:
		return chroma.LiteralStringOther, false
	case word[0] == '#':
		return chroma.NameConstant, false
	case strings.ContainsRune(word, '$'):
		return chroma.LiteralNumber, false
	}
	value := word
	if value[0] == '+' || value[0] == '-' {
		value = value[1:]
	}
	if len(value) > 1 && (value[0] == '.' || value[0] == ',') {
		value = value[1:]
	}
	if len(value) > 0 && rebolDigit(value[0]) {
		return chroma.LiteralNumber, false
	}
	base, _, refined := strings.Cut(word, "/")
	base = strings.ToLower(base)
	if kind, ok := rebolWords[base]; ok {
		return kind, base == "comment" && !refined
	}
	return chroma.Text, false
}

func rebolDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func rebolBinaryOpen(text string) int {
	i := 0
	for i < min(len(text), 2) && rebolDigit(text[i]) {
		i++
	}
	if strings.HasPrefix(text[i:], "#{") {
		return i + 2
	}
	return 0
}

func rebolQuotedEnd(line string, i int) int {
	for i < len(line) {
		ch := line[i]
		i++
		if ch == '"' {
			break
		}
		if ch == '^' && i < len(line) {
			i++
		}
	}
	return i
}

func rebolTagStart(ch byte) bool {
	return ch == '/' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

func rebolTagEnd(line string, i int) int {
	for i < len(line) {
		ch := line[i]
		i++
		if ch == '>' {
			break
		}
		if ch == '"' {
			i = rebolQuotedEnd(line, i)
		}
	}
	return i
}

func rebolFoldRegions(value string) []FoldRegion {
	var regions []FoldRegion
	var starts []int
	var state rebolState
	for row := 0; value != ""; row++ {
		end, next := syntaxLineBounds(value)
		line := value[:end]
		state = scanREBOLLine(line, state, func(start, end int, kind chroma.TokenType) {
			if kind != chroma.Punctuation || end-start != 1 {
				return
			}
			switch line[start] {
			case '[':
				starts = append(starts, row)
			case ']':
				if n := len(starts); n > 0 {
					if header := starts[n-1]; header < row {
						regions = append(regions, FoldRegion{Header: header, LastLine: row, Level: n})
					}
					starts = starts[:n-1]
				}
			}
		})
		value = value[next:]
	}
	return sortedFoldRegions(regions)
}
