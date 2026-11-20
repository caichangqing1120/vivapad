package syntax

import (
	_ "embed"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Word list from Notepad++ v8.9.8.1, PowerEditor/src/langs.model.xml.
//
//go:embed csound_opcodes.txt
var csoundOpcodes string

var csoundWords = func() map[string]chroma.TokenType {
	words := make(map[string]chroma.TokenType)
	for _, word := range strings.Fields("sr kr ar ksmps nchnls") {
		words[word] = chroma.KeywordDeclaration
	}
	for _, word := range strings.Fields(csoundOpcodes) {
		words[word] = chroma.Keyword
	}
	return words
}()

type csoundLexer struct{ config chroma.Config }

var _ = lexers.Register(&csoundLexer{config: chroma.Config{Name: "CSound", Aliases: []string{"csound"}}})

func (l *csoundLexer) Config() *chroma.Config                         { return &l.config }
func (l *csoundLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *csoundLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*csoundLexer) AnalyseText(string) float32                       { return 0 }

func (*csoundLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanCSoundLine), nil
}

type CSoundHighlighter struct{ *lineHighlighter[bool] }

func NewCSoundHighlighter() *CSoundHighlighter {
	h := newLineHighlighter(scanCSoundLine)
	h.color = csoundTokenColor
	return &CSoundHighlighter{h}
}

func scanCSoundLine(line string, continuedComment bool, emit func(int, int, chroma.TokenType)) bool {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	if continuedComment {
		style(0, len(line), chroma.CommentSingle)
		return strings.HasSuffix(line, `\`)
	}
	for i := 0; i < len(line); {
		start := i
		ch := line[i]
		i++
		switch {
		case ch == ';':
			style(start, len(line), chroma.CommentSingle)
			return strings.HasSuffix(line, `\`)
		case csoundDigit(ch) || ch == '.' && i < len(line) && csoundDigit(line[i]):
			for i < len(line) && csoundWordByte(line[i]) {
				i++
			}
			style(start, i, chroma.LiteralNumber)
		case csoundWordByte(ch) || ch == '%' || ch == '@' || ch == '$':
			for i < len(line) && csoundWordByte(line[i]) {
				i++
			}
			if emit == nil {
				continue
			}
			if kind, ok := csoundWords[line[start:i]]; ok {
				style(start, i, kind)
				continue
			}
			switch ch {
			case 'p', 'a', 'k', 'i':
				style(start, i, chroma.NameVariable)
			case 'g':
				style(start, i, chroma.NameVariableGlobal)
			}
		case strings.IndexByte("*/-+()=^[]<&>,|~:", ch) >= 0:
			style(start, i, chroma.Operator)
		}
	}
	return false
}

func csoundDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func csoundWordByte(ch byte) bool {
	return csoundDigit(ch) || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch == '.' || ch == '_' || ch == '?'
}

func csoundMode(mode string) bool {
	switch strings.ToLower(mode) {
	case ".orc", ".sco", ".csd":
		return true
	}
	return false
}

func csoundFoldRegions(value string) []FoldRegion {
	var regions []FoldRegion
	var starts []int
	continued := false
	for row := 0; value != ""; row++ {
		end, next := syntaxLineBounds(value)
		line := value[:end]
		first := len(line) - len(strings.TrimLeft(line, " \t"))
		continued = scanCSoundLine(line, continued, func(start, end int, kind chroma.TokenType) {
			if start != first || kind != chroma.Keyword {
				return
			}
			switch line[start:end] {
			case "instr":
				starts = append(starts, row)
			case "endin":
				if n := len(starts); n > 0 {
					regions = append(regions, FoldRegion{Header: starts[n-1], LastLine: row, Level: n})
					starts = starts[:n-1]
				}
			}
		})
		value = value[next:]
	}
	return sortedFoldRegions(regions)
}
