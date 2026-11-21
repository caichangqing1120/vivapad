package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
)

type txt2tagsLexer struct{ config chroma.Config }

var _ = lexers.Register(&txt2tagsLexer{config: chroma.Config{Name: "txt2tags"}})

func (l *txt2tagsLexer) Config() *chroma.Config                         { return &l.config }
func (l *txt2tagsLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *txt2tagsLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*txt2tagsLexer) AnalyseText(string) float32                       { return 0 }

func (*txt2tagsLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanTxt2tagsLine), nil
}

type txt2tagsState struct {
	fence byte
	blank bool
	seen  bool
}

type Txt2tagsHighlighter struct {
	*lineHighlighter[txt2tagsState]
}

func NewTxt2tagsHighlighter() *Txt2tagsHighlighter {
	h := newLineHighlighter(scanTxt2tagsLine)
	h.color = txt2tagsTokenColor
	return &Txt2tagsHighlighter{h}
}

func txt2tagsTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.GenericHeading, chroma.GenericStrong:
		return tokenColor(chroma.Keyword)
	case chroma.GenericEmph, chroma.GenericUnderline, chroma.NameAttribute:
		return gfx.RGB(31, 111, 124), true
	case chroma.GenericDeleted:
		return gfx.RGB(139, 75, 147), true
	case chroma.CommentPreproc:
		return tokenColor(chroma.Keyword)
	default:
		return tokenColor(kind)
	}
}

func scanTxt2tagsLine(line string, state txt2tagsState, emit func(int, int, chroma.TokenType)) txt2tagsState {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	trimmed := strings.Trim(line, " \t")
	first := len(line) - len(strings.TrimLeft(line, " \t"))
	allowRule := !state.seen || state.blank
	state.seen, state.blank = true, trimmed == ""
	if state.fence != 0 {
		style(0, len(line), txt2tagsFenceStyle(state.fence))
		if txt2tagsFence(trimmed) == state.fence {
			state.fence = 0
		}
		return state
	}
	if state.blank {
		return state
	}
	if fence := txt2tagsFence(trimmed); fence != 0 {
		state.fence = fence
		style(first, len(line), txt2tagsFenceStyle(fence))
		return state
	}
	switch {
	case txt2tagsDirective(trimmed, "%!preproc"), txt2tagsDirective(trimmed, "%!postproc"):
		style(first, len(line), chroma.CommentPreproc)
		return state
	case strings.HasPrefix(trimmed, "%!"):
		style(first, len(line), chroma.KeywordNamespace)
		return state
	case trimmed[0] == '%':
		style(first, len(line), chroma.CommentSingle)
		return state
	case txt2tagsHeading(trimmed):
		style(first, len(line), chroma.GenericHeading)
		return state
	case allowRule && len(trimmed) >= 20 && strings.ContainsRune("-*_", rune(trimmed[0])) &&
		strings.Trim(trimmed, trimmed[:1]) == "":
		style(first, len(line), chroma.NameTag)
		return state
	case strings.HasPrefix(trimmed, "| "), strings.HasPrefix(trimmed, "|| "), strings.HasPrefix(trimmed, ": "):
		style(first, len(line), chroma.NameTag)
		return state
	case len(trimmed) > 3 && (trimmed[3] == ' ' || trimmed[3] == '\t') && txt2tagsFence(trimmed[:3]) != 0:
		style(first, len(line), chroma.LiteralStringBacktick)
		return state
	}
	if emit == nil {
		return state
	}
	if marker := txt2tagsListMarker(trimmed); marker != 0 {
		style(first, first+marker, chroma.NameTag)
		first += marker
	}
	scanTxt2tagsInline(line, first, style)
	return state
}

func txt2tagsFence(s string) byte {
	switch s {
	case "```", `"""`, "'''", "%%%":
		return s[0]
	default:
		return 0
	}
}

func txt2tagsFenceStyle(fence byte) chroma.TokenType {
	if fence == '%' {
		return chroma.CommentMultiline
	}
	return chroma.LiteralStringBacktick
}

func txt2tagsDirective(line, prefix string) bool {
	tail, ok := strings.CutPrefix(line, prefix)
	return ok && (tail == "" || strings.ContainsRune(":( \t", rune(tail[0])))
}

func txt2tagsHeading(line string) bool {
	mark := line[0]
	if mark != '=' && mark != '+' {
		return false
	}
	if line[len(line)-1] == ']' {
		if i := strings.LastIndexByte(line, '['); i > 0 {
			line = strings.TrimRight(line[:i], " \t")
		}
	}
	left := len(line) - len(strings.TrimLeft(line, line[:1]))
	right := len(line) - len(strings.TrimRight(line, line[:1]))
	return left <= 6 && left == right && left+right < len(line) && strings.TrimSpace(line[left:len(line)-right]) != ""
}

func txt2tagsListMarker(s string) int {
	if len(s) > 1 && (s[0] == '-' || s[0] == '+') && (s[1] == ' ' || s[1] == '\t') {
		return 2
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(s) && s[i] == '.' && (s[i+1] == ' ' || s[i+1] == '\t') {
		return i + 2
	}
	return 0
}

var txt2tagsInline = [...]struct {
	open, close string
	kind        chroma.TokenType
}{
	{"**", "**", chroma.GenericStrong}, {"//", "//", chroma.GenericEmph},
	{"__", "__", chroma.GenericUnderline}, {"--", "--", chroma.GenericDeleted},
	{"``", "``", chroma.LiteralStringBacktick}, {"\"\"", "\"\"", chroma.LiteralStringBacktick},
	{"''", "''", chroma.LiteralStringBacktick}, {"[", "]", chroma.NameAttribute}, {"![", "]", chroma.NameAttribute},
}

func scanTxt2tagsInline(line string, i int, emit func(int, int, chroma.TokenType)) {
	var exhausted [len(txt2tagsInline)]bool
	for i < len(line) {
		if line[i] == '\\' {
			i += 2
			continue
		}
		start := i
		for j, rule := range txt2tagsInline {
			if exhausted[j] || !strings.HasPrefix(line[i:], rule.open) {
				continue
			}
			content := i + len(rule.open)
			if content >= len(line) || txt2tagsSpace(line[content]) {
				break
			}
			end := txt2tagsInlineEnd(line, content, rule.close)
			if end < 0 {
				exhausted[j] = true
				break
			}
			emit(i, end, rule.kind)
			i = end
			break
		}
		if i == start {
			i++
		}
	}
}

func txt2tagsInlineEnd(line string, start int, close string) int {
	for i := start; i < len(line); i++ {
		if line[i] == '\\' {
			i++
		} else if i > start && !txt2tagsSpace(line[i-1]) && strings.HasPrefix(line[i:], close) {
			return i + len(close)
		}
	}
	return -1
}

func txt2tagsSpace(ch byte) bool { return ch == ' ' || ch == '\t' }
