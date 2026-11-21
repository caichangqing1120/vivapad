package syntax

import (
	_ "embed"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
)

// The case-sensitive nnCrontab word groups in Notepad++ v8.9.8.1 langs.model.xml.
//
//go:embed nncrontab_words.txt
var nncrontabWordData string

var nncrontabWords = func() map[string]chroma.TokenType {
	words := make(map[string]chroma.TokenType)
	data := strings.ReplaceAll(strings.TrimSpace(nncrontabWordData), "\r\n", "\n")
	kinds := [...]chroma.TokenType{chroma.Keyword, chroma.NameBuiltin, chroma.KeywordPseudo}
	for i, group := range strings.SplitN(data, "\n\n", len(kinds)) {
		for _, word := range strings.Fields(group) {
			if _, exists := words[word]; !exists {
				words[word] = kinds[i]
			}
		}
	}
	return words
}()

type nncrontabLexer struct{ config chroma.Config }

var _ = lexers.Register(&nncrontabLexer{config: chroma.Config{Name: "Nncrontab"}})

func (l *nncrontabLexer) Config() *chroma.Config                         { return &l.config }
func (l *nncrontabLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *nncrontabLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*nncrontabLexer) AnalyseText(string) float32                       { return 0 }

func (*nncrontabLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanNncrontabLine), nil
}

func nncrontabTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.NameBuiltin:
		return gfx.RGB(31, 111, 124), true
	case chroma.KeywordPseudo, chroma.LiteralStringInterpol:
		return gfx.RGB(113, 76, 144), true
	case chroma.Operator:
		return tokenColor(chroma.LiteralNumber)
	default:
		return tokenColor(kind)
	}
}

func scanNncrontabLine(line string, _ struct{}, emit func(int, int, chroma.TokenType)) struct{} {
	for i := 0; i < len(line); {
		start := i
		ch := line[i]
		i++
		switch {
		case strings.HasPrefix(line[start:], "#("), strings.HasPrefix(line[start:], ")#"):
			emit(start, len(line), chroma.NameTag)
			return struct{}{}
		case ch == '#', ch == '\\' && i < len(line) && (line[i] == ' ' || line[i] == '\t'):
			emit(start, len(line), chroma.CommentSingle)
			return struct{}{}
		case ch == '"':
			i = scanNncrontabString(line, start, emit)
		case ch == '%':
			i = nncrontabSubstitutionEnd(line, i, '%', true)
			emit(start, i, chroma.LiteralStringInterpol)
		case ch == '<' && i < len(line) && line[i] == '%':
			i = nncrontabSubstitutionEnd(line, i+1, '>', false)
			emit(start, i, chroma.LiteralStringInterpol)
		case ch == '*':
			emit(start, i, chroma.Operator)
		case nncrontabDigit(ch):
			for i < len(line) && nncrontabDigit(line[i]) {
				i++
			}
			emit(start, i, chroma.LiteralNumberInteger)
		case nncrontabLetter(ch), ch == '<':
			for i < len(line) && nncrontabWordByte(line[i]) {
				i++
			}
			if kind, ok := nncrontabWords[line[start:i]]; ok {
				emit(start, i, kind)
			}
		}
	}
	return struct{}{}
}

func scanNncrontabString(line string, start int, emit func(int, int, chroma.TokenType)) int {
	i := start + 1
	for i < len(line) {
		switch line[i] {
		case '"':
			if line[i-1] != '\\' {
				emit(start, i+1, chroma.LiteralStringDouble)
				return i + 1
			}
		case '%':
			if i > start {
				emit(start, i, chroma.LiteralStringDouble)
			}
			end := nncrontabSubstitutionEnd(line, i+1, '%', false)
			emit(i, end, chroma.LiteralStringInterpol)
			i, start = end, end
			continue
		}
		i++
	}
	if start < i {
		emit(start, i, chroma.LiteralStringDouble)
	}
	return i
}

func nncrontabSubstitutionEnd(line string, start int, close byte, escaped bool) int {
	for i := start; i < len(line); i++ {
		if line[i] == close && (!escaped || line[i-1] != '\\') {
			return i + 1
		}
	}
	return len(line)
}

func nncrontabLetter(ch byte) bool { return ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' }
func nncrontabDigit(ch byte) bool  { return ch >= '0' && ch <= '9' }
func nncrontabWordByte(ch byte) bool {
	return nncrontabLetter(ch) || nncrontabDigit(ch) || strings.ContainsRune("_-/$.<>@", rune(ch))
}
