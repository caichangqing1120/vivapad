package syntax

import (
	_ "embed"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
)

// Hollywood keyword groups from Notepad++ v8.9.8.1 langs.model.xml.
//
//go:embed hollywood_words.txt
var hollywoodWordData string

var hollywoodWords = func() map[string]chroma.TokenType {
	words := make(map[string]chroma.TokenType)
	styles := [...]chroma.TokenType{chroma.Keyword, chroma.NameBuiltin, chroma.NameFunction, chroma.NameFunctionMagic}
	groups := strings.SplitN(strings.TrimSpace(strings.ReplaceAll(hollywoodWordData, "\r\n", "\n")), "\n\n", len(styles))
	for index, group := range groups {
		for _, word := range strings.Fields(group) {
			words[strings.ToLower(word)] = styles[index]
		}
	}
	return words
}()

type hollywoodLexer struct{ config chroma.Config }

var _ = lexers.Register(&hollywoodLexer{config: chroma.Config{Name: "Hollywood"}})

func (l *hollywoodLexer) Config() *chroma.Config                         { return &l.config }
func (l *hollywoodLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *hollywoodLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*hollywoodLexer) AnalyseText(string) float32                       { return 0 }

func (*hollywoodLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanHollywoodLine), nil
}

type hollywoodState uint8

const (
	hollywoodCode hollywoodState = iota
	hollywoodComment
	hollywoodString
)

type HollywoodHighlighter struct {
	*lineHighlighter[hollywoodState]
}

func NewHollywoodHighlighter() *HollywoodHighlighter {
	h := newLineHighlighter(scanHollywoodLine)
	h.color = hollywoodTokenColor
	return &HollywoodHighlighter{h}
}

func hollywoodTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.NameBuiltin:
		return gfx.RGB(31, 111, 124), true
	case chroma.NameFunctionMagic, chroma.NameConstant:
		return gfx.RGB(139, 75, 147), true
	case chroma.CommentPreproc:
		return tokenColor(chroma.Keyword)
	default:
		return tokenColor(kind)
	}
}

func scanHollywoodLine(line string, state hollywoodState, emit func(int, int, chroma.TokenType)) hollywoodState {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	for i := 0; i < len(line); {
		start := i
		if state == hollywoodCode {
			switch {
			case strings.HasPrefix(line[i:], "/*"):
				state, i = hollywoodComment, i+2
			case strings.HasPrefix(line[i:], "[["):
				state, i = hollywoodString, i+2
			}
		}
		if state != hollywoodCode {
			close, kind := "*/", chroma.CommentMultiline
			if state == hollywoodString {
				close, kind = "]]", chroma.LiteralString
			}
			end := hollywoodBlockEnd(line, i, close)
			if end < 0 {
				style(start, len(line), kind)
				return state
			}
			style(start, end, kind)
			i, state = end, hollywoodCode
			continue
		}

		ch := line[i]
		i++
		switch {
		case ch == ';':
			style(start, len(line), chroma.CommentSingle)
			return hollywoodCode
		case ch == '"':
			for i < len(line) {
				next := line[i]
				i++
				if next == '\\' && i < len(line) {
					i++
				} else if next == '"' {
					break
				}
			}
			style(start, i, chroma.LiteralStringDouble)
		case ch == '#' || ch == '@':
			for i < len(line) && hollywoodIdentifier(line[i]) {
				i++
			}
			kind := chroma.NameConstant
			if ch == '@' {
				kind = chroma.CommentPreproc
			}
			style(start, i, kind)
		case ch == '$' || ch == '0' && i < len(line) && (line[i] == 'x' || line[i] == 'X'):
			if ch == '0' {
				i++
			}
			for i < len(line) && (hollywoodDigit(line[i]) || line[i]|0x20 >= 'a' && line[i]|0x20 <= 'f') {
				i++
			}
			style(start, i, chroma.LiteralNumberHex)
		case hollywoodDigit(ch) || ch == '.' && i < len(line) && hollywoodDigit(line[i]):
			for i < len(line) && (hollywoodDigit(line[i]) || line[i] == '.') {
				i++
			}
			style(start, i, chroma.LiteralNumber)
		case hollywoodIdentifier(ch):
			for i < len(line) && hollywoodIdentifier(line[i]) {
				i++
			}
			if emit != nil {
				if kind, ok := hollywoodWords[strings.ToLower(line[start:i])]; ok {
					style(start, i, kind)
				}
			}
		case strings.IndexByte("%&'()*+,-/:<=>?[\\]^`{|}~", ch) >= 0:
			style(start, i, chroma.Operator)
		}
	}
	return state
}

func hollywoodBlockEnd(line string, start int, closer string) int {
	quoted := false
	for i := start; i < len(line); i++ {
		if line[i] == '"' && (i == 0 || line[i-1] != '\\') {
			quoted = !quoted
		}
		if !quoted && strings.HasPrefix(line[i:], closer) {
			return i + len(closer)
		}
	}
	return -1
}

func hollywoodDigit(c byte) bool { return c >= '0' && c <= '9' }

func hollywoodIdentifier(c byte) bool {
	cLower := c | 0x20
	return cLower >= 'a' && cLower <= 'z' || hollywoodDigit(c) || strings.IndexByte("!$._", c) >= 0
}

func hollywoodFoldRegions(value string) []FoldRegion {
	var state hollywoodState
	var starts []foldStart
	var regions []FoldRegion
	for line := 0; value != ""; line++ {
		end, next := syntaxLineBounds(value)
		source := value[:end]
		state = scanHollywoodLine(source, state, func(start, end int, kind chroma.TokenType) {
			if kind != chroma.Keyword {
				return
			}
			switch strings.ToLower(source[start:end]) {
			case "function":
				starts = append(starts, foldStart{line: line, level: len(starts) + 1})
			case "endfunction":
				if len(starts) > 0 {
					prev := starts[len(starts)-1]
					starts = starts[:len(starts)-1]
					if prev.line < line {
						regions = append(regions, FoldRegion{Header: prev.line, LastLine: line, Level: prev.level})
					}
				}
			}
		})
		value = value[next:]
	}
	return sortedFoldRegions(regions)
}
