package syntax

import (
	_ "embed"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// FreeBASIC words and directives from Notepad++ v8.9.8.1 langs.model.xml.
//
//go:embed freebasic_words.txt
var freeBasicWordData string

var freeBasicWords = func() map[string]chroma.TokenType {
	words := make(map[string]chroma.TokenType)
	for _, word := range strings.Fields(freeBasicWordData) {
		kind := chroma.Keyword
		if strings.HasPrefix(word, "#") {
			kind = chroma.CommentPreproc
		}
		words[word] = kind
	}
	return words
}()

type freeBasicLexer struct{ config chroma.Config }

var _ = lexers.Register(&freeBasicLexer{config: chroma.Config{Name: "FreeBasic"}})

func (l *freeBasicLexer) Config() *chroma.Config                         { return &l.config }
func (l *freeBasicLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *freeBasicLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*freeBasicLexer) AnalyseText(string) float32                       { return 0 }

func (*freeBasicLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanFreeBasicLine), nil
}

type FreeBasicHighlighter struct{ *lineHighlighter[bool] }

func NewFreeBasicHighlighter() *FreeBasicHighlighter {
	return &FreeBasicHighlighter{newLineHighlighter(scanFreeBasicLine)}
}

func scanFreeBasicLine(line string, inComment bool, emit func(int, int, chroma.TokenType)) bool {
	return scanBasicLine(line, inComment, emit, false)
}

func scanBasicLine(line string, inComment bool, emit func(int, int, chroma.TokenType), pure bool) bool {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	i := 0
	first := len(line) - len(strings.TrimLeft(line, " \t"))
	comment, words := byte('\''), freeBasicWords
	if pure {
		comment, words = ';', pureBasicWords
	}
	for i < len(line) {
		start := i
		if inComment || strings.HasPrefix(line[i:], "/'") {
			if !inComment {
				i += 2
			}
			end := strings.Index(line[i:], "'/")
			if end < 0 {
				style(start, len(line), chroma.CommentMultiline)
				return true
			}
			i += end + 2
			style(start, i, chroma.CommentMultiline)
			inComment = false
			continue
		}

		ch := line[i]
		i++
		switch {
		case ch == comment:
			kind := chroma.CommentSingle
			if !pure && i < len(line) && line[i] == '$' {
				kind = chroma.CommentPreproc
			}
			style(start, len(line), kind)
			return false
		case ch == '"' || pure && ch == '~' && i < len(line) && line[i] == '"':
			escaped := ch == '~'
			if escaped {
				i++
			}
			for i < len(line) {
				if escaped && line[i] == '\\' && i+1 < len(line) {
					i += 2
					continue
				}
				if line[i] != '"' {
					i++
					continue
				}
				i++
				if i >= len(line) || line[i] != '"' {
					break
				}
				i++
			}
			style(start, i, chroma.LiteralStringDouble)
		case pure && (ch == '$' || ch == '%'):
			kind, base := chroma.LiteralNumberHex, byte(16)
			if ch == '%' {
				kind, base = chroma.LiteralNumberBin, 2
			}
			for i < len(line) && freeBasicDigit(line[i], base) {
				i++
			}
			if i == start+1 {
				kind = chroma.Operator
			}
			style(start, i, kind)
		case !pure && ch == '&' && i < len(line) && strings.ContainsRune("hHoObB", rune(line[i])):
			kind, base := chroma.LiteralNumberHex, byte(16)
			switch line[i] {
			case 'o', 'O':
				kind, base = chroma.LiteralNumberOct, 8
			case 'b', 'B':
				kind, base = chroma.LiteralNumberBin, 2
			}
			i++
			for i < len(line) && freeBasicDigit(line[i], base) {
				i++
			}
			style(start, i, kind)
		case freeBasicDigit(ch, 10) || ch == '.' && i < len(line) && freeBasicDigit(line[i], 10):
			i = freeBasicDecimalEnd(line, start)
			style(start, i, chroma.LiteralNumber)
		case ch == '#' || freeBasicWordByte(ch):
			for i < len(line) && freeBasicWordByte(line[i]) {
				i++
			}
			if !pure && i < len(line) && line[i] == '$' && words[strings.ToLower(line[start:i+1])] == chroma.Keyword {
				i++
			}
			word := strings.ToLower(line[start:i])
			if !pure && word == "rem" {
				style(start, len(line), chroma.CommentSingle)
				return false
			}
			if ch != '#' && start == first && i < len(line) && line[i] == ':' {
				i++
				style(start, i, chroma.NameLabel)
			} else if kind, ok := words[word]; ok {
				style(start, i, kind)
			} else if ch == '#' && i > start+1 {
				style(start, i, chroma.NameConstant)
			}
		case strings.IndexByte("+-*/\\^=&|<>!~%:.,()[]{}$", ch) >= 0:
			style(start, i, chroma.Operator)
		}
	}
	return inComment
}

func freeBasicDigit(ch, base byte) bool {
	return ch >= '0' && ch <= '9' && ch-'0' < base ||
		base == 16 && (ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F')
}

func freeBasicWordByte(ch byte) bool {
	return freeBasicDigit(ch, 10) || ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

func freeBasicDecimalEnd(line string, i int) int {
	for i < len(line) && freeBasicDigit(line[i], 10) {
		i++
	}
	if i < len(line) && line[i] == '.' {
		i++
		for i < len(line) && freeBasicDigit(line[i], 10) {
			i++
		}
	}
	if i < len(line) && strings.ContainsRune("EeDd", rune(line[i])) {
		exponent := i + 1
		if exponent < len(line) && (line[exponent] == '+' || line[exponent] == '-') {
			exponent++
		}
		if exponent < len(line) && freeBasicDigit(line[exponent], 10) {
			i = exponent + 1
			for i < len(line) && freeBasicDigit(line[i], 10) {
				i++
			}
		}
	}
	return i
}

func freeBasicFoldRegions(value string) []FoldRegion {
	type opening struct {
		line int
		kind string
	}
	var regions []FoldRegion
	var stack []opening
	inComment := false
	for row := 0; value != ""; row++ {
		end, next := syntaxLineBounds(value)
		line := value[:end]
		first := len(line) - len(strings.TrimLeft(line, " \t"))
		inComment = scanFreeBasicLine(line, inComment, func(start, end int, kind chroma.TokenType) {
			if kind != chroma.Keyword || start != first {
				return
			}
			word := strings.ToLower(line[start:end])
			switch word {
			case "function", "sub", "enum", "type", "union", "property", "destructor", "constructor":
				stack = append(stack, opening{row, word})
			case "end":
				closing := strings.TrimLeft(line[end:], " \t")
				at := 0
				for at < len(closing) && freeBasicWordByte(closing[at]) {
					at++
				}
				if n := len(stack); n > 0 && strings.EqualFold(stack[n-1].kind, closing[:at]) {
					regions = append(regions, FoldRegion{Header: stack[n-1].line, LastLine: row, Level: n})
					stack = stack[:n-1]
				}
			}
		})
		value = value[next:]
	}
	return sortedFoldRegions(regions)
}
