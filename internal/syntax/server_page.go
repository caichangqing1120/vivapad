package syntax

import (
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/components"
)

type pageScript uint8

const (
	pageDefault pageScript = iota
	pageVB
	pageJava
	pageJavaScript
	pageCSharp
	pageCSS
)

var pageScriptLexers = [...]string{"", "VB.net", "Java", "JavaScript", "C#", "CSS"}

type pageBlock uint8

const (
	pageMarkup pageBlock = iota
	pageCode
	pageDirective
	pageServerComment
)

type serverPageState struct {
	server, raw, nextRaw pageScript
	block                pageBlock
	markupEnd            string
	directive            string
	quotedAttr           byte
	directiveQuote       byte
	inTag, attrValue     bool
	blockComment         bool
	directiveOverflow    bool
}

type serverPageLexer struct {
	config chroma.Config
	script pageScript
}

var (
	_ = lexers.Register(&serverPageLexer{config: chroma.Config{Name: "ASP", Aliases: []string{"asp"}}, script: pageVB})
	_ = lexers.Register(&serverPageLexer{config: chroma.Config{Name: "JSP", Aliases: []string{"jsp"}}, script: pageJava})
)

func (l *serverPageLexer) Config() *chroma.Config                         { return &l.config }
func (l *serverPageLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *serverPageLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*serverPageLexer) AnalyseText(string) float32                       { return 0 }
func (l *serverPageLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, serverPageScanner(l.script)), nil
}

type ServerPageHighlighter struct {
	mode string
	line *lineHighlighter[serverPageState]
}

func (h *ServerPageHighlighter) ForText(mode, value string) func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun {
	if h.line == nil || h.mode != mode {
		script := pageVB
		if mode == ".jsp" {
			script = pageJava
		}
		h.mode, h.line = mode, newLineHighlighter(serverPageScanner(script))
	}
	return h.line.ForText(value)
}

var (
	pageVBNumber   = regexp.MustCompile(`(?i)^[0-9]+(?:e[0-9]+)?(?:us|ui|ul|[silfdr])?$`)
	pageJavaNumber = regexp.MustCompile(`(?i)^(?:[0-9][0-9_]*(?:e[0-9][0-9_]*)?[lfd]?|0x[0-9a-f][0-9a-f_]*l?|0b[01][01_]*l?)$`)
)

func parsePageLanguage(text string) pageScript {
	for text = strings.TrimSpace(text); text != ""; text = strings.TrimSpace(text) {
		end := 0
		for end < len(text) && pageNameByte(text[end]) {
			end++
		}
		if end == 0 {
			text = text[1:]
			continue
		}
		name := text[:end]
		text = strings.TrimSpace(text[end:])
		if text == "" || text[0] != '=' {
			continue
		}
		text = strings.TrimSpace(text[1:])
		if text == "" {
			break
		}
		var value string
		if quote := text[0]; quote == '\'' || quote == '"' {
			end = strings.IndexByte(text[1:], quote)
			if end < 0 {
				return pageDefault
			}
			value, text = text[1:1+end], text[2+end:]
		} else {
			end = strings.IndexAny(text, " \t\r\n%")
			if end < 0 {
				end = len(text)
			}
			value, text = text[:end], text[end:]
		}
		if strings.EqualFold(name, "language") {
			return namedPageLanguage(value)
		}
	}
	return pageDefault
}

func namedPageLanguage(value string) pageScript {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "vbscript", "vb", "vb.net", "vbnet", "visualbasic":
		return pageVB
	case "java":
		return pageJava
	case "javascript", "jscript", "js":
		return pageJavaScript
	case "c#", "cs", "csharp":
		return pageCSharp
	default:
		return pageDefault
	}
}

func serverPageScanner(defaultScript pageScript) scanLine[serverPageState] {
	return func(line string, state serverPageState, emit func(int, int, chroma.TokenType)) serverPageState {
		if state.server == pageDefault {
			state.server = defaultScript
		}
		line = strings.TrimSuffix(line, "\r")
		paint := func(start, end int, kind chroma.TokenType) {
			if emit != nil && end > start {
				emit(start, end, kind)
			}
		}
		for i := 0; i < len(line); {
			start, rest := i, line[i:]
			switch {
			case state.block == pageServerComment:
				i = pageSpanEnd(line, i, "--%>")
				paint(start, i, chroma.CommentMultiline)
				if strings.HasSuffix(line[start:i], "--%>") {
					state.block = pageMarkup
				}
			case state.block == pageDirective:
				var closed bool
				i, closed = endPageDirective(line, i, &state.directiveQuote)
				paint(start, i, chroma.CommentPreproc)
				// Only the language attribute survives the directive. Do not retain
				// arbitrarily large unfinished attributes in every line's state.
				if !state.directiveOverflow {
					if i-start+len(state.directive) >= 4096 {
						state.directive, state.directiveOverflow = "", true
					} else {
						state.directive += line[start:i] + " "
					}
				}
				if closed {
					if script := parsePageLanguage(state.directive); script != pageDefault {
						state.server = script
					}
					state.block, state.directive, state.directiveOverflow = pageMarkup, "", false
				}
			case state.block == pageCode:
				i = scanPageCode(line, i, state.server, &state, emit)
			case state.markupEnd != "":
				i = pageSpanEnd(line, i, state.markupEnd)
				kind := chroma.CommentMultiline
				if state.markupEnd == "]]>" {
					kind = chroma.CommentPreproc
				}
				paint(start, i, kind)
				if strings.HasSuffix(line[start:i], state.markupEnd) {
					state.markupEnd = ""
				}
			case strings.HasPrefix(rest, "<%--"):
				i = pageSpanEnd(line, i+4, "--%>")
				paint(start, i, chroma.CommentMultiline)
				if !strings.HasSuffix(line[start+4:i], "--%>") {
					state.block = pageServerComment
				}
			case strings.HasPrefix(rest, "<%@"):
				state.block = pageDirective
				i += 3
				paint(start, i, chroma.CommentPreproc)
			case strings.HasPrefix(rest, "<%"):
				state.block = pageCode
				i += 2
				if i < len(line) && strings.ContainsRune("=!:#", rune(line[i])) {
					i++
				}
				paint(start, i, chroma.CommentPreproc)
			case state.raw != pageDefault:
				endTag := "script"
				if state.raw == pageCSS {
					endTag = "style"
				}
				if pageClosingTag(rest, endTag) {
					state.raw = pageDefault
					continue
				}
				i++
				for i < len(line) && !strings.HasPrefix(line[i:], "<%") && !pageClosingTag(line[i:], endTag) {
					i++
				}
				paintPageCode(line[start:i], start, state.raw, emit)
			case state.quotedAttr != 0:
				for i < len(line) && !strings.HasPrefix(line[i:], "<%") {
					ch := line[i]
					i++
					if ch == state.quotedAttr {
						state.quotedAttr = 0
						break
					}
				}
				paint(start, i, chroma.LiteralString)
			case state.inTag:
				i++
				switch ch := line[start]; {
				case ch == '>':
					state.inTag, state.attrValue = false, false
					state.raw, state.nextRaw = state.nextRaw, pageDefault
				case ch == '/' && strings.HasPrefix(rest, "/>"):
					state.nextRaw = pageDefault
				case ch == '=':
					state.attrValue = true
				case ch == ' ' || ch == '\t':
				case ch == '"' || ch == '\'':
					state.quotedAttr, state.attrValue = ch, false
					paint(start, i, chroma.LiteralString)
				case state.attrValue:
					for i < len(line) && !strings.ContainsRune(" \t>", rune(line[i])) && !strings.HasPrefix(line[i:], "<%") {
						i++
					}
					paint(start, i, chroma.LiteralString)
					state.attrValue = false
				case pageNameByte(ch):
					for i < len(line) && pageNameByte(line[i]) {
						i++
					}
					paint(start, i, chroma.NameAttribute)
				}
			case strings.HasPrefix(rest, "<!--") || strings.HasPrefix(rest, "<![CDATA["):
				state.markupEnd = "-->"
				if strings.HasPrefix(rest, "<![CDATA[") {
					state.markupEnd = "]]>"
				}
			case strings.HasPrefix(rest, "<!") || strings.HasPrefix(rest, "<?"):
				i = pageSpanEnd(line, i+2, ">")
				paint(start, i, chroma.CommentPreproc)
			case line[i] == '<':
				i++
				closing := i < len(line) && line[i] == '/'
				if closing {
					i++
				}
				nameStart := i
				if i < len(line) && pageLetter(line[i]) {
					for i < len(line) && pageNameByte(line[i]) {
						i++
					}
					paint(nameStart, i, chroma.NameTag)
					state.inTag = true
					if !closing {
						switch name := line[nameStart:i]; {
						case strings.EqualFold(name, "script"):
							state.nextRaw = pageJavaScript
						case strings.EqualFold(name, "style"):
							state.nextRaw = pageCSS
						}
					}
				}
			default:
				i++
			}
		}
		return state
	}
}

func endPageDirective(line string, i int, quote *byte) (int, bool) {
	for ; i < len(line); i++ {
		ch := line[i]
		if *quote != 0 {
			if ch == *quote {
				*quote = 0
			}
		} else if ch == '\'' || ch == '"' {
			*quote = ch
		} else if ch == '%' && i+1 < len(line) && line[i+1] == '>' {
			return i + 2, true
		}
	}
	return len(line), false
}

func scanPageCode(line string, i int, script pageScript, state *serverPageState, emit func(int, int, chroma.TokenType)) int {
	paint := func(start, end int, kind chroma.TokenType) {
		if emit != nil && end > start {
			emit(start, end, kind)
		}
	}
	start := i
	rest := line[i:]
	switch {
	case state.blockComment:
		i = pageSpanEnd(line, i, "*/")
		paint(start, i, chroma.CommentMultiline)
		state.blockComment = !strings.HasSuffix(line[start:i], "*/")
		return i
	case strings.HasPrefix(rest, "%>"):
		state.block = pageMarkup
		paint(i, i+2, chroma.CommentPreproc)
		return i + 2
	case script != pageVB && strings.HasPrefix(rest, "/*"):
		i = pageSpanEnd(line, i+2, "*/")
		paint(start, i, chroma.CommentMultiline)
		state.blockComment = !strings.HasSuffix(line[start+2:i], "*/")
		return i
	case pageLineComment(rest, script):
		if end := strings.Index(rest, "%>"); end >= 0 {
			i += end
		} else {
			i = len(line)
		}
		paint(start, i, chroma.CommentSingle)
		return i
	case rest[0] == '"' || script != pageVB && rest[0] == '\'':
		quote := rest[0]
		i++
		for i < len(line) {
			ch := line[i]
			i++
			if script != pageVB && ch == '\\' && i < len(line) {
				i++
			} else if ch == quote {
				if script == pageVB && i < len(line) && line[i] == quote {
					i++
					continue
				}
				break
			}
		}
		paint(start, i, chroma.LiteralString)
		return i
	default:
		i++
		for i < len(line) && !strings.HasPrefix(line[i:], "%>") && line[i] != '"' &&
			(script == pageVB || line[i] != '\'' && !strings.HasPrefix(line[i:], "/*")) && !pageLineComment(line[i:], script) {
			i++
		}
		paintPageCode(line[start:i], start, script, emit)
		return i
	}
}

func pageLineComment(rest string, script pageScript) bool {
	if script == pageVB {
		return rest[0] == '\''
	}
	return strings.HasPrefix(rest, "//")
}

func paintPageCode(value string, start int, script pageScript, emit func(int, int, chroma.TokenType)) {
	if emit == nil || strings.TrimSpace(value) == "" || len(value) > components.CodeEditorMaxHighlightedLine {
		return
	}
	lexer := lexers.Get(pageScriptLexers[script])
	if lexer == nil {
		return
	}
	iterator, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root", Nested: true}, value)
	if err != nil {
		return
	}
	end := start + len(value)
	for t := iterator(); t != chroma.EOF && start < end; t = iterator() {
		stop := min(end, start+len(t.Value))
		if stop > start {
			kind := t.Type
			// The embedded lexers can return numeric words as names.
			if kind == chroma.Name && (script == pageVB && pageVBNumber.MatchString(t.Value) ||
				script == pageJava && pageJavaNumber.MatchString(t.Value)) {
				kind = chroma.LiteralNumber
			}
			emit(start, stop, kind)
		}
		start = stop
	}
}

func pageSpanEnd(line string, i int, marker string) int {
	if end := strings.Index(line[i:], marker); end >= 0 {
		return i + end + len(marker)
	}
	return len(line)
}

func pageClosingTag(rest, name string) bool {
	end := len(name) + 2
	return len(rest) >= end && strings.HasPrefix(rest, "</") && strings.EqualFold(rest[2:end], name) &&
		(len(rest) == end || strings.ContainsRune(" \t>", rune(rest[end])))
}

func pageLetter(ch byte) bool { ch |= 32; return ch >= 'a' && ch <= 'z' }
func pageNameByte(ch byte) bool {
	return pageLetter(ch) || ch >= '0' && ch <= '9' || ch == ':' || ch == '-' || ch == '_' || ch == '.'
}
