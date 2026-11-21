package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
)

// Word lists from Notepad++ v8.9.8.1 langs.model.xml.
var visualPrologWords = func() map[string]chroma.TokenType {
	words := make(map[string]chroma.TokenType)
	for _, group := range []struct {
		words, prefix string
		kind          chroma.TokenType
	}{
		{"goal namespace interface class implement open inherits supports resolve delegate monitor constants domains predicates constructors properties clauses facts", "", chroma.KeywordDeclaration},
		{"guard language stdcall apicall c thiscall prolog digits if then elseif else foreach do try catch finally erroneous failure procedure determ multi nondeterm anyflow and or externally from div mod rem quot in orelse otherwise", "", chroma.Keyword},
		{"include bininclude requires orrequires if then else elseif endif error message export externally options", "#", chroma.CommentPreproc},
		{"short detail end exception withdomain", "@", chroma.CommentSpecial},
	} {
		for _, word := range strings.Fields(group.words) {
			words[group.prefix+word] = group.kind
		}
	}
	return words
}()

type visualPrologLexer struct{ config chroma.Config }

var _ = lexers.Register(&visualPrologLexer{config: chroma.Config{Name: "Visual Prolog"}})

func (l *visualPrologLexer) Config() *chroma.Config                         { return &l.config }
func (l *visualPrologLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *visualPrologLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*visualPrologLexer) AnalyseText(string) float32                       { return 0 }
func (*visualPrologLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanVisualPrologLine), nil
}

// Each immutable frame can be shared by the line-start cache.
type visualPrologFrame struct {
	embedded bool
	outer    *visualPrologFrame
}

type visualPrologState struct {
	frame       *visualPrologFrame
	comments    int
	verbatimEnd rune
}

type VisualPrologHighlighter struct {
	*lineHighlighter[visualPrologState]
}

func NewVisualPrologHighlighter() *VisualPrologHighlighter {
	h := newLineHighlighter(scanVisualPrologLine)
	h.color = visualPrologTokenColor
	return &VisualPrologHighlighter{h}
}

func visualPrologTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.KeywordDeclaration, chroma.CommentPreproc:
		return gfx.RGB(113, 76, 144), true
	case chroma.NameVariable, chroma.CommentSpecial:
		return gfx.RGB(31, 111, 124), true
	case chroma.NameVariableMagic:
		return gfx.RGB(94, 118, 126), true
	case chroma.LiteralStringEscape:
		return gfx.RGB(139, 75, 147), true
	case chroma.Error:
		return gfx.RGB(177, 39, 58), true
	default:
		return tokenColor(kind)
	}
}

func scanVisualPrologLine(line string, state visualPrologState, emit func(int, int, chroma.TokenType)) visualPrologState {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	for i := 0; i < len(line); {
		start := i
		remaining := line[i:]
		embedded := state.frame != nil && state.frame.embedded
		if state.verbatimEnd != 0 {
			i, state.verbatimEnd = visualPrologVerbatim(line, i, state.verbatimEnd)
			style(start, i, chroma.LiteralString)
			continue
		}
		if state.comments != 0 || strings.HasPrefix(remaining, "/*") {
			kind := chroma.CommentMultiline
			if embedded {
				kind = chroma.LiteralStringOther
			}
			if state.comments == 0 {
				state.comments++
				i += 2
			}
			for i < len(line) && state.comments > 0 {
				switch {
				case strings.HasPrefix(line[i:], "/*"):
					state.comments++
					i += 2
				case strings.HasPrefix(line[i:], "*/"):
					state.comments--
					i += 2
				default:
					i++
				}
			}
			if embedded {
				style(start, i, kind)
			} else {
				visualPrologComment(line, start, i, kind, style)
			}
			continue
		}
		switch {
		case strings.HasPrefix(remaining, "[|"):
			state.frame = &visualPrologFrame{embedded: true, outer: state.frame}
			i += 2
			style(start, i, chroma.LiteralStringOther)
		case embedded && strings.HasPrefix(remaining, "|]"):
			state.frame = state.frame.outer
			i += 2
			style(start, i, chroma.LiteralStringOther)
		case strings.HasPrefix(remaining, "{|"):
			state.frame = &visualPrologFrame{outer: state.frame}
			i += 2
			style(start, i, chroma.StringInterpol)
		case !embedded && strings.HasPrefix(remaining, "|}"):
			if state.frame != nil {
				state.frame = state.frame.outer
			}
			i += 2
			if i < len(line) && line[i] == ':' {
				i = visualPrologIdentifierEnd(line, i+1)
			}
			style(start, i, chroma.StringInterpol)
		case embedded:
			i++
			for i < len(line) && !strings.HasPrefix(line[i:], "[|") && !strings.HasPrefix(line[i:], "|]") &&
				!strings.HasPrefix(line[i:], "{|") && !strings.HasPrefix(line[i:], "/*") {
				i++
			}
			style(start, i, chroma.LiteralStringOther)
		case line[i] == '%':
			visualPrologComment(line, i, len(line), chroma.CommentSingle, style)
			return state
		case line[i] == '\'' || line[i] == '"':
			i = visualPrologQuoted(line, i, style)
		case line[i] == '@':
			next, width := utf8.DecodeRuneInString(line[i+1:])
			if closer := visualPrologCloser(next); width != 0 && closer != 0 {
				i, state.verbatimEnd = visualPrologVerbatim(line, i+1+width, closer)
				style(start, i, chroma.LiteralString)
			} else {
				i++
			}
		case line[i] == '#':
			i++
			for i < len(line) && line[i] >= 'a' && line[i] <= 'z' {
				i++
			}
			if kind, ok := visualPrologWords[line[start:i]]; ok {
				style(start, i, kind)
			}
		case visualPrologDigit(line[i]) || line[i] == '.' && i+1 < len(line) && visualPrologDigit(line[i+1]):
			end, kind := visualPrologNumber(line, i)
			i = end
			style(start, i, kind)
		default:
			first, width := utf8.DecodeRuneInString(remaining)
			i += width
			if first == '_' || unicode.IsLetter(first) {
				i = visualPrologIdentifierEnd(line, i)
				word := line[start:i]
				switch {
				case first == '_':
					style(start, i, chroma.NameVariableMagic)
				case unicode.IsUpper(first):
					style(start, i, chroma.NameVariable)
				case word == "end":
					next := strings.TrimLeft(line[i:], " \t")
					if kind, ok := visualPrologWords[next[:visualPrologIdentifierEnd(next, 0)]]; ok {
						style(start, i, kind)
					}
				default:
					if kind, ok := visualPrologWords[word]; ok {
						style(start, i, kind)
					}
				}
			}
		}
	}
	return state
}

func visualPrologIdentifierEnd(line string, i int) int {
	for i < len(line) {
		r, width := utf8.DecodeRuneInString(line[i:])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			break
		}
		i += width
	}
	return i
}

func visualPrologComment(line string, start, end int, kind chroma.TokenType, style func(int, int, chroma.TokenType)) {
	for i := start; i < end; i++ {
		if line[i] != '@' {
			continue
		}
		last := i + 1
		for last < end && line[last]|0x20 >= 'a' && line[last]|0x20 <= 'z' {
			last++
		}
		if visualPrologWords[line[i:last]] == chroma.CommentSpecial {
			style(start, i, kind)
			style(i, last, chroma.CommentSpecial)
			start = last
		}
		i = last - 1
	}
	style(start, end, kind)
}

func visualPrologCloser(open rune) rune {
	switch open {
	case '<':
		return '>'
	case '>':
		return '<'
	case '(':
		return ')'
	case ')':
		return '('
	case '[':
		return ']'
	case ']':
		return '['
	case '{':
		return '}'
	case '}':
		return '{'
	case '_', '.', ',', ';':
		return 0
	}
	if open <= 0x7fff && (unicode.IsPunct(open) || unicode.IsSymbol(open)) {
		return open
	}
	return 0
}

func visualPrologVerbatim(line string, i int, closer rune) (int, rune) {
	for i < len(line) {
		r, width := utf8.DecodeRuneInString(line[i:])
		i += width
		if r == closer {
			next, width := utf8.DecodeRuneInString(line[i:])
			if next != closer {
				return i, 0
			}
			i += width
		}
	}
	return i, closer
}

func visualPrologQuoted(line string, start int, style func(int, int, chroma.TokenType)) int {
	quote := line[start]
	for i := start + 1; i < len(line); {
		switch line[i] {
		case quote:
			style(start, i+1, chroma.LiteralString)
			return i + 1
		case '\\':
			style(start, i, chroma.LiteralString)
			end, kind := i+1, chroma.Error
			if end < len(line) {
				r, width := utf8.DecodeRuneInString(line[end:])
				end += width
				if strings.ContainsRune("\"'\\nlrt", r) {
					kind = chroma.LiteralStringEscape
				} else if r == 'u' {
					begin := end
					for end < len(line) && end-begin < 4 && visualPrologHex(line[end]) {
						end++
					}
					if end-begin == 4 {
						kind = chroma.LiteralStringEscape
					}
				}
			}
			style(i, end, kind)
			i, start = end, end
		default:
			i++
		}
	}
	style(start, len(line), chroma.LiteralString)
	return len(line)
}

func visualPrologDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
func visualPrologHex(ch byte) bool   { return visualPrologDigit(ch) || ch|0x20 >= 'a' && ch|0x20 <= 'f' }

func visualPrologNumber(line string, start int) (int, chroma.TokenType) {
	i, kind := start, chroma.LiteralNumber
	if i+1 < len(line) && line[i] == '0' {
		switch line[i+1] | 0x20 {
		case 'x':
			kind = chroma.LiteralNumberHex
		case 'o':
			kind = chroma.LiteralNumberOct
		}
		if kind != chroma.LiteralNumber {
			i += 2
			for i < len(line) && (line[i] == '_' || visualPrologHex(line[i])) {
				i++
			}
			return i, kind
		}
	}
	if line[i] == '.' {
		i++
	}
	for i < len(line) && (visualPrologDigit(line[i]) || line[i] == '_') {
		i++
	}
	return i, kind
}
