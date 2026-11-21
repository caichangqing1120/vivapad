package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

type innoSection uint8

const (
	innoData innoSection = iota
	innoCode
	innoMessages
)

type innoState struct {
	section innoSection
	closer  byte
}

type innoLexer struct{ config chroma.Config }

var _ = lexers.Register(&innoLexer{config: chroma.Config{Name: "Inno Setup", Aliases: []string{"inno"}}})

func (l *innoLexer) Config() *chroma.Config                         { return &l.config }
func (l *innoLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *innoLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*innoLexer) AnalyseText(string) float32                       { return 0 }

func (*innoLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanInnoTokens), nil
}

type InnoHighlighter struct{ *lineHighlighter[innoState] }

func NewInnoHighlighter() *InnoHighlighter {
	return &InnoHighlighter{newLineHighlighter(scanInnoTokens)}
}

func scanInnoTokens(line string, state innoState, emit func(int, int, chroma.TokenType)) innoState {
	next, _ := scanInnoLine(line, state, emit)
	return next
}

// scanInnoLine reports the next line state and whether this line starts a section.
// Fold discovery and the editor's line index do not need the styled byte ranges.
func scanInnoLine(line string, state innoState, emit func(int, int, chroma.TokenType)) (innoState, bool) {
	header := false
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	block := func(start, body int) int {
		end, width := strings.IndexByte(line[body:], '}'), 1
		if state.closer == ')' {
			end, width = strings.Index(line[body:], "*)"), 2
		}
		if end < 0 {
			style(start, len(line), chroma.CommentMultiline)
			return len(line)
		}
		end += body + width
		state.closer = 0
		style(start, end, chroma.CommentMultiline)
		return end
	}
	first := len(line) - len(strings.TrimLeft(line, " \t"))
	i := 0
	if state.closer != 0 {
		i = block(0, 0)
	}
	for i < len(line) {
		start := i
		if i == first {
			switch line[i] {
			case ';':
				if state.section != innoCode {
					style(i, len(line), chroma.CommentSingle)
					return state, header
				}
			case '[':
				end := innoWordEnd(line, i+1)
				if end < len(line) && line[end] == ']' {
					name := strings.ToLower(line[i+1 : end])
					i = end + 1
					if innoSections[name] {
						header = true
						style(start, i, chroma.NameTag)
						switch name {
						case "code":
							state.section = innoCode
						case "messages", "custommessages":
							state.section = innoMessages
						default:
							state.section = innoData
						}
					}
					continue
				}
			case '#':
				begin := len(line) - len(strings.TrimLeft(line[i+1:], " \t"))
				i = innoWordEnd(line, begin)
				if emit != nil && (i == len(line) || line[i] == ' ' || line[i] == '\t') && innoDirectives[strings.ToLower(line[begin:i])] {
					style(start, i, chroma.CommentPreproc)
				}
				continue
			}
		}
		rest := line[i:]
		switch {
		case state.section == innoCode && rest[0] == '{':
			state.closer = '}'
			i = block(i, i+1)
		case state.section == innoCode && strings.HasPrefix(rest, "(*"):
			state.closer = ')'
			i = block(i, i+2)
		case state.section == innoCode && strings.HasPrefix(rest, "//"):
			style(i, len(line), chroma.CommentSingle)
			return state, header
		case state.section != innoCode && rest[0] == '{':
			if strings.HasPrefix(rest, "{{") {
				i += 2
			} else if end := strings.IndexByte(rest[1:], '}'); end >= 0 {
				i += end + 2
				style(start, i, chroma.LiteralStringInterpol)
			} else {
				return state, header
			}
		case state.section != innoMessages && (rest[0] == '\'' || rest[0] == '"'):
			kind := chroma.LiteralStringSingle
			if rest[0] == '"' {
				kind = chroma.LiteralStringDouble
			}
			if end := strings.IndexByte(rest[1:], rest[0]); end >= 0 {
				i += end + 2
			} else {
				i = len(line)
			}
			style(start, i, kind)
		case state.section != innoMessages && innoWordByte(rest[0]):
			i = innoWordEnd(line, i)
			if emit == nil {
				continue
			}
			word := strings.ToLower(line[start:i])
			if state.section == innoCode {
				if innoPascal[word] {
					style(start, i, chroma.Keyword)
				}
			} else {
				tail := strings.TrimLeft(line[i:], " \t")
				if tail != "" && (tail[0] == '=' && innoAssignments[word] || tail[0] == ':' && innoParameters[word]) {
					style(start, i, chroma.Keyword)
				}
			}
		default:
			i++
		}
	}
	return state, header
}

func innoWordByte(ch byte) bool {
	return ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'
}

func innoWordEnd(line string, start int) int {
	end := start
	for end < len(line) && innoWordByte(line[end]) {
		end++
	}
	return end
}
