package syntax

import (
	_ "embed"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
)

// OScript word groups from Notepad++ v8.9.8.1 langs.model.xml.
//
//go:embed oscript_words.txt
var oscriptWordData string

var oscriptWords, oscriptObjects = func() (map[string]chroma.TokenType, map[string]bool) {
	kinds := [...]chroma.TokenType{chroma.Keyword, chroma.KeywordConstant, chroma.OperatorWord, chroma.KeywordType, chroma.NameFunction}
	words, objects := make(map[string]chroma.TokenType), make(map[string]bool)
	data := strings.ReplaceAll(strings.TrimSpace(oscriptWordData), "\r\n", "\n")
	for i, group := range strings.SplitN(data, "\n\n", len(kinds)+1) {
		for _, word := range strings.Fields(group) {
			if i == len(kinds) {
				objects[word] = true
			} else if _, exists := words[word]; !exists {
				words[word] = kinds[i]
			}
		}
	}
	return words, objects
}()

type oscriptState struct{ block, doc bool }

type oscriptLexer struct{ config chroma.Config }

var _ = lexers.Register(&oscriptLexer{config: chroma.Config{Name: "OScript"}})

func (l *oscriptLexer) Config() *chroma.Config                         { return &l.config }
func (l *oscriptLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *oscriptLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*oscriptLexer) AnalyseText(string) float32                       { return 0 }

func (*oscriptLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanOScriptLine), nil
}

type OScriptHighlighter struct{ *lineHighlighter[oscriptState] }

func NewOScriptHighlighter() *OScriptHighlighter {
	h := newLineHighlighter(scanOScriptLine)
	h.color = oscriptTokenColor
	return &OScriptHighlighter{h}
}

func oscriptTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.NameClass, chroma.NameProperty, chroma.NameFunctionMagic, chroma.NameVariableGlobal:
		return gfx.RGB(113, 76, 144), true
	case chroma.OperatorWord:
		return tokenColor(chroma.Keyword)
	default:
		return tokenColor(kind)
	}
}

func scanOScriptLine(line string, state oscriptState, emit func(int, int, chroma.TokenType)) oscriptState {
	return scanOScript(line, state, emit, nil)
}

func scanOScript(line string, state oscriptState, emit func(int, int, chroma.TokenType), commentFold func(bool)) oscriptState {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	first := len(line) - len(strings.TrimLeft(line, " \t"))
	if state.doc {
		style(first, len(line), chroma.CommentSpecial)
		state.doc = oscriptDirective(line[first:]) != "#endif"
		return state
	}

	i := 0
	for i < len(line) {
		start := i
		if state.block || strings.HasPrefix(line[i:], "/*") {
			if !state.block {
				i += 2
				if commentFold != nil {
					commentFold(true)
				}
			}
			end := strings.Index(line[i:], "*/")
			if end < 0 {
				style(start, len(line), chroma.CommentMultiline)
				return oscriptState{block: true}
			}
			i += end + 2
			style(start, i, chroma.CommentMultiline)
			state.block = false
			if commentFold != nil {
				commentFold(false)
			}
			continue
		}

		ch := line[i]
		i++
		switch {
		case strings.HasPrefix(line[start:], "//"):
			style(start, len(line), chroma.CommentSingle)
			return state
		case ch == '#' && start == first:
			kind := chroma.CommentPreproc
			if fields := strings.Fields(line[start:]); len(fields) > 1 && strings.EqualFold(fields[0], "#ifdef") && strings.EqualFold(fields[1], "doc") {
				kind, state.doc = chroma.CommentSpecial, true
			}
			style(start, len(line), kind)
			return state
		case ch == '\'' || ch == '"':
			kind := chroma.LiteralStringDouble
			if ch == '\'' {
				kind = chroma.LiteralStringSingle
			}
			for i < len(line) {
				c := line[i]
				i++
				if c == ch {
					if i >= len(line) || line[i] != ch {
						break
					}
					i++
				}
			}
			style(start, i, kind)
		case ch == '$':
			if i < len(line) && line[i] == '$' {
				i++
			}
			for i < len(line) && oscriptWordByte(line[i]) {
				i++
			}
			style(start, i, chroma.NameVariableGlobal)
		case oscriptDigit(ch) || strings.IndexByte(".+-", ch) >= 0 && i < len(line) && oscriptDigit(line[i]):
			i = oscriptNumberEnd(line, start)
			style(start, i, chroma.LiteralNumber)
		case oscriptWordByte(ch):
			for i < len(line) && oscriptWordByte(line[i]) {
				i++
			}
			if emit == nil {
				continue
			}
			word := strings.ToLower(line[start:i])
			kind := oscriptWords[word]
			var after byte
			if i < len(line) {
				after = line[i]
			}
			switch {
			case start == first && after == ':':
				i++
				kind = chroma.NameLabel
			case start > 0 && line[start-1] == '.':
				kind = chroma.NameProperty
				if after == '(' {
					kind = chroma.NameFunctionMagic
				}
			case after == '.' && oscriptObjects[word]:
				kind = chroma.NameClass
			case after == '(' && kind != chroma.Keyword && kind != chroma.OperatorWord && kind != chroma.NameFunction:
				kind = chroma.NameFunctionMagic
			}
			if kind != 0 {
				style(start, i, kind)
			}
		case strings.IndexByte("%^&*()-+={}[]:;<>,/?!.~|\\", ch) >= 0:
			style(start, i, chroma.Operator)
		}
	}
	return state
}

func oscriptDirective(line string) string {
	if !strings.HasPrefix(line, "#") {
		return ""
	}
	for i := 1; i < len(line); i++ {
		if !oscriptWordByte(line[i]) {
			return strings.ToLower(line[:i])
		}
	}
	return strings.ToLower(line)
}

func oscriptDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func oscriptWordByte(ch byte) bool {
	return oscriptDigit(ch) || ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

func oscriptNumberEnd(line string, i int) int {
	if line[i] == '+' || line[i] == '-' {
		i++
	}
	for i < len(line) && oscriptDigit(line[i]) {
		i++
	}
	if i < len(line) && line[i] == '.' {
		i++
		for i < len(line) && oscriptDigit(line[i]) {
			i++
		}
	}
	if i < len(line) && (line[i] == 'e' || line[i] == 'E') {
		next := i + 1
		if next < len(line) && (line[next] == '+' || line[next] == '-') {
			next++
		}
		if next < len(line) && oscriptDigit(line[next]) {
			i = next + 1
			for i < len(line) && oscriptDigit(line[i]) {
				i++
			}
		}
	}
	return i
}

func oscriptFoldRegions(value string) []FoldRegion {
	type opening struct {
		line   int
		closer string
	}
	var regions []FoldRegion
	var stack []opening
	var state oscriptState
	for row := 0; value != ""; row++ {
		end, next := syntaxLineBounds(value)
		line := value[:end]
		first := len(line) - len(strings.TrimLeft(line, " \t"))
		typeEnd := -1
		open := func(closer string) { stack = append(stack, opening{row, closer}) }
		closeFold := func(closer string) {
			if n := len(stack); n > 0 && stack[n-1].closer == closer {
				if header := stack[n-1].line; header < row {
					regions = append(regions, FoldRegion{Header: header, LastLine: row, Level: n})
				}
				stack = stack[:n-1]
			}
		}
		state = scanOScript(line, state, func(start, end int, kind chroma.TokenType) {
			switch kind {
			case chroma.CommentPreproc, chroma.CommentSpecial:
				switch oscriptDirective(line[start:end]) {
				case "#ifdef", "#ifndef":
					if !state.doc {
						open("#endif")
					}
				case "#endif":
					closeFold("#endif")
				}
			case chroma.KeywordType:
				if start == first {
					typeEnd = end
				}
			case chroma.Keyword:
				word := strings.ToLower(line[start:end])
				if start != first && !(word == "function" && typeEnd >= 0 && strings.TrimSpace(line[typeEnd:start]) == "") {
					return
				}
				switch word {
				case "function", "if", "for", "switch", "while":
					open("end")
				case "repeat":
					open("until")
				case "end", "until":
					closeFold(word)
				}
			}
		}, func(opening bool) {
			if opening {
				open("*/")
			} else {
				closeFold("*/")
			}
		})
		value = value[next:]
	}
	return sortedFoldRegions(regions)
}
