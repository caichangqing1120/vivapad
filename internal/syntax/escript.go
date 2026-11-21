package syntax

import (
	_ "embed"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
)

// ESCRIPT word lists from Notepad++ v8.9.8.1 langs.model.xml.
//
//go:embed escript_words.txt
var escriptWordData string

var escriptWords = func() map[string]chroma.TokenType {
	kinds := [...]chroma.TokenType{chroma.NameBuiltin, chroma.Keyword, chroma.KeywordDeclaration}
	words := make(map[string]chroma.TokenType)
	for i, group := range strings.SplitN(strings.TrimSpace(escriptWordData), "\n", len(kinds)) {
		for _, word := range strings.Fields(group) {
			word = strings.ToLower(word)
			if _, exists := words[word]; !exists {
				words[word] = kinds[i]
			}
		}
	}
	return words
}()

type escriptLexer struct{ config chroma.Config }

var _ = lexers.Register(&escriptLexer{config: chroma.Config{Name: "ESCRIPT"}})

func (l *escriptLexer) Config() *chroma.Config                         { return &l.config }
func (l *escriptLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *escriptLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*escriptLexer) AnalyseText(string) float32                       { return 0 }
func (*escriptLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanESCRIPTLine), nil
}

type escriptState uint8

const (
	escriptCode escriptState = iota
	escriptBlockComment
	escriptLineComment
	escriptString
)

type ESCRIPTHighlighter struct{ *lineHighlighter[escriptState] }

func NewESCRIPTHighlighter() *ESCRIPTHighlighter {
	h := newLineHighlighter(scanESCRIPTLine)
	h.color = escriptTokenColor
	return &ESCRIPTHighlighter{h}
}

func escriptTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.NameBuiltin:
		return gfx.RGB(31, 111, 124), true
	case chroma.KeywordDeclaration:
		return tokenColor(chroma.NameFunction)
	default:
		return tokenColor(kind)
	}
}

func scanESCRIPTLine(line string, state escriptState, emit func(int, int, chroma.TokenType)) escriptState {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	if state == escriptLineComment {
		style(0, len(line), chroma.CommentSingle)
		if strings.HasSuffix(line, "\\") {
			return escriptLineComment
		}
		return escriptCode
	}
	for i := 0; i < len(line); {
		start := i
		if state == escriptCode {
			switch {
			case strings.HasPrefix(line[i:], "/*"):
				state, i = escriptBlockComment, i+2
			case strings.HasPrefix(line[i:], "//"):
				style(start, len(line), chroma.CommentSingle)
				if strings.HasSuffix(line, "\\") {
					return escriptLineComment
				}
				return escriptCode
			case line[i] == '"':
				state, i = escriptString, i+1
			}
		}
		switch state {
		case escriptBlockComment:
			close := strings.Index(line[i:], "*/")
			if close < 0 {
				style(start, len(line), chroma.CommentMultiline)
				return state
			}
			i, state = i+close+2, escriptCode
			style(start, i, chroma.CommentMultiline)
		case escriptString:
			for i < len(line) {
				ch := line[i]
				i++
				if ch == '\\' && i < len(line) && (line[i] == '"' || line[i] == '\\') {
					i++
				} else if ch == '"' {
					state = escriptCode
					break
				}
			}
			style(start, i, chroma.LiteralStringDouble)
		default:
			ch := line[i]
			i++
			switch {
			case escriptDigit(ch) || ch == '.' && i < len(line) && escriptDigit(line[i]):
				for i < len(line) && escriptDigit(line[i]) {
					i++
				}
				if ch != '.' && i+1 < len(line) && line[i] == '.' && escriptDigit(line[i+1]) {
					i++
					for i < len(line) && escriptDigit(line[i]) {
						i++
					}
				}
				style(start, i, chroma.LiteralNumber)
			case escriptWord(ch) || ch == '#':
				for i < len(line) && escriptWord(line[i]) {
					i++
				}
				if kind, ok := escriptWords[strings.ToLower(line[start:i])]; ok {
					style(start, i, kind)
				}
			case strings.IndexByte("+-*/=<>&|!?:", ch) >= 0:
				style(start, i, chroma.Operator)
			case ch == '{' || ch == '}':
				style(start, i, chroma.Punctuation)
			}
		}
	}
	return state
}

func escriptDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func escriptWord(ch byte) bool {
	return escriptDigit(ch) || ch == '_' || ch|0x20 >= 'a' && ch|0x20 <= 'z'
}

func escriptFoldRegions(value string) []FoldRegion {
	type block struct {
		line  int
		close string
	}
	var state escriptState
	var open []block
	var regions []FoldRegion
	for line := 0; value != ""; line++ {
		end, next := syntaxLineBounds(value)
		source := value[:end]
		previous := ""
		state = scanESCRIPTLine(source, state, func(start, end int, kind chroma.TokenType) {
			if kind != chroma.KeywordDeclaration {
				return
			}
			word := strings.ToLower(source[start:end])
			switch word {
			case "program", "function", "while", "for", "foreach", "case", "if":
				if word != "if" || previous != "else" {
					open = append(open, block{line: line, close: "end" + word})
				}
			default:
				if n := len(open); n > 0 && word == open[n-1].close {
					start := open[n-1].line
					open = open[:n-1]
					if start < line {
						regions = append(regions, FoldRegion{Header: start, LastLine: line, Level: n})
					}
				}
			}
			previous = word
		})
		value = value[next:]
	}
	return sortedFoldRegions(regions)
}
