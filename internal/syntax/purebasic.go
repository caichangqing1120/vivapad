package syntax

import (
	_ "embed"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// PureBasic keyword and compiler groups from Notepad++ v8.9.8.1 langs.model.xml.
//
//go:embed purebasic_words.txt
var pureBasicWordData string

var pureBasicWords = func() map[string]chroma.TokenType {
	words := make(map[string]chroma.TokenType)
	data := strings.ReplaceAll(strings.TrimSpace(pureBasicWordData), "\r\n", "\n")
	regular, compiler, _ := strings.Cut(data, "\n\n")
	for _, word := range strings.Fields(regular) {
		words[word] = chroma.Keyword
	}
	for _, word := range strings.Fields(compiler) {
		words[word] = chroma.CommentPreproc
	}
	return words
}()

type pureBasicLexer struct{ config chroma.Config }

var _ = lexers.Register(&pureBasicLexer{config: chroma.Config{Name: "PureBasic"}})

func (l *pureBasicLexer) Config() *chroma.Config                         { return &l.config }
func (l *pureBasicLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *pureBasicLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*pureBasicLexer) AnalyseText(string) float32                       { return 0 }

func (*pureBasicLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanPureBasicLine), nil
}

type PureBasicHighlighter struct{ *lineHighlighter[bool] }

func NewPureBasicHighlighter() *PureBasicHighlighter {
	return &PureBasicHighlighter{newLineHighlighter(scanPureBasicLine)}
}

func scanPureBasicLine(line string, inComment bool, emit func(int, int, chroma.TokenType)) bool {
	return scanBasicLine(line, inComment, emit, true)
}

func pureBasicFoldRegions(value string) []FoldRegion {
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
		inComment = scanPureBasicLine(line, inComment, func(start, end int, kind chroma.TokenType) {
			if kind != chroma.Keyword || start != first {
				return
			}
			word := strings.ToLower(line[start:end])
			switch word {
			case "procedure", "enumeration", "interface", "structure":
				stack = append(stack, opening{row, word})
			case "endprocedure", "endenumeration", "endinterface", "endstructure":
				if n := len(stack); n > 0 && stack[n-1].kind == word[len("end"):] {
					regions = append(regions, FoldRegion{Header: stack[n-1].line, LastLine: row, Level: n})
					stack = stack[:n-1]
				}
			}
		})
		value = value[next:]
	}
	return sortedFoldRegions(regions)
}
