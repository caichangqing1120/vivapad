package syntax

import (
	_ "embed"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
)

// The five nonempty AviSynth word groups in Notepad++ v8.9.8.1.
//
//go:embed avisynth_words.txt
var avsWordData string

var avsWords = parseAVSWords(avsWordData)

func parseAVSWords(data string) map[string]chroma.TokenType {
	kinds := [...]chroma.TokenType{chroma.Keyword, chroma.NameFunction, chroma.NameDecorator, chroma.NameBuiltin, chroma.NameProperty}
	words := make(map[string]chroma.TokenType)
	data = strings.ReplaceAll(strings.TrimSpace(data), "\r\n", "\n")
	for i, group := range strings.SplitN(data, "\n\n", len(kinds)) {
		for _, word := range strings.Fields(group) {
			word = strings.ToLower(word)
			if _, exists := words[word]; !exists {
				words[word] = kinds[i]
			}
		}
	}
	return words
}

type avsState struct {
	closer string
	depth  int
}

type avsLexer struct{ config chroma.Config }

var _ = lexers.Register(&avsLexer{config: chroma.Config{Name: "AviSynth", Aliases: []string{"avs"}}})

func (l *avsLexer) Config() *chroma.Config                         { return &l.config }
func (l *avsLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *avsLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*avsLexer) AnalyseText(string) float32                       { return 0 }

func (*avsLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanAVSLine), nil
}

type AviSynthHighlighter struct{ *lineHighlighter[avsState] }

func NewAviSynthHighlighter() *AviSynthHighlighter {
	h := newLineHighlighter(scanAVSLine)
	h.color = avsTokenColor
	return &AviSynthHighlighter{h}
}

func avsTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.NameFunction:
		return gfx.RGB(49, 92, 167), true
	case chroma.NameDecorator, chroma.NameBuiltin, chroma.NameProperty:
		return gfx.RGB(113, 76, 144), true
	default:
		return tokenColor(kind)
	}
}

func scanAVSLine(line string, state avsState, emit func(int, int, chroma.TokenType)) avsState {
	return scanAVS(line, state, emit, nil)
}

// Fold events are produced only for code braces and whole block comments.
func scanAVS(line string, state avsState, emit func(int, int, chroma.TokenType), fold func(bool)) avsState {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	consume := func(start, i int) int {
		if state.depth == 0 {
			if end := strings.Index(line[i:], state.closer); end >= 0 {
				i += end + len(state.closer)
				state.closer = ""
			} else {
				i = len(line)
			}
			style(start, i, chroma.LiteralStringDouble)
			return i
		}
		opener := "/*"
		if state.closer == "*]" {
			opener = "[*"
		}
		for i < len(line) && state.depth != 0 {
			switch {
			case strings.HasPrefix(line[i:], opener):
				state.depth++
				i += 2
			case strings.HasPrefix(line[i:], state.closer):
				state.depth--
				i += 2
			default:
				i++
			}
		}
		if state.depth == 0 {
			state.closer = ""
			if fold != nil {
				fold(false)
			}
		}
		style(start, i, chroma.CommentMultiline)
		return i
	}

	i := 0
	if state.closer != "" {
		i = consume(0, 0)
	}
	for i < len(line) {
		start := i
		ch := line[i]
		i++
		switch {
		case ch == '#':
			style(start, len(line), chroma.CommentSingle)
			return state
		case strings.HasPrefix(line[start:], "/*") || strings.HasPrefix(line[start:], "[*"):
			state = avsState{closer: "*/", depth: 1}
			if ch == '[' {
				state.closer = "*]"
			}
			if fold != nil {
				fold(true)
			}
			i = consume(start, start+2)
		case ch == '"':
			state.closer = `"`
			if strings.HasPrefix(line[start:], `"""`) {
				state.closer = `"""`
			}
			i = consume(start, start+len(state.closer))
		case avsDigit(ch) || ch == '.' && i < len(line) && avsDigit(line[i]):
			for i < len(line) && avsDigit(line[i]) {
				i++
			}
			if ch != '.' && i < len(line) && line[i] == '.' {
				i++
				for i < len(line) && avsDigit(line[i]) {
					i++
				}
			}
			style(start, i, chroma.LiteralNumber)
		case avsWordByte(ch):
			for i < len(line) && avsWordByte(line[i]) {
				i++
			}
			if emit != nil {
				if kind, ok := avsWords[strings.ToLower(line[start:i])]; ok {
					style(start, i, kind)
				}
			}
		case ch == '{' || ch == '}':
			style(start, i, chroma.Operator)
			if fold != nil {
				fold(ch == '{')
			}
		case strings.IndexByte("+-*/%[]()=<>!&|?:.,", ch) >= 0:
			style(start, i, chroma.Operator)
		}
	}
	return state
}

func avsDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func avsWordByte(ch byte) bool {
	return avsDigit(ch) || ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

func avsFoldRegions(value string) []FoldRegion {
	var regions []FoldRegion
	var starts []int
	var state avsState
	for row := 0; value != ""; row++ {
		end, next := syntaxLineBounds(value)
		state = scanAVS(value[:end], state, nil, func(opening bool) {
			if opening {
				starts = append(starts, row)
			} else if n := len(starts); n > 0 {
				if header := starts[n-1]; header < row {
					regions = append(regions, FoldRegion{Header: header, LastLine: row, Level: n})
				}
				starts = starts[:n-1]
			}
		})
		value = value[next:]
	}
	return sortedFoldRegions(regions)
}
