package syntax

import (
	"slices"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/components"
)

// Resource scripts use the C-family lexer with a separate keyword list in
// Notepad++ v8.9.8.1 (PowerEditor/src/langs.model.xml, language rc).
const resourceWords = `ACCELERATORS ALT AUTO3STATE AUTOCHECKBOX AUTORADIOBUTTON BEGIN BITMAP BLOCK BUTTON CAPTION CHARACTERISTICS CHECKBOX CLASS COMBOBOX CONTROL CTEXT CURSOR DEFPUSHBUTTON DIALOG DIALOGEX DISCARDABLE EDITTEXT END EXSTYLE FONT GROUPBOX ICON LANGUAGE LISTBOX LTEXT MENU MENUEX MENUITEM MESSAGETABLE POPUP PUSHBUTTON RADIOBUTTON RCDATA RTEXT SCROLLBAR SEPARATOR SHIFT STATE3 STRINGTABLE STYLE TEXTINCLUDE VALUE VERSION VERSIONINFO VIRTKEY`

var resourceKeywords = strings.Fields(resourceWords)

var resourceConfig = chroma.Config{Name: "Windows Resource"}

type resourceLexer struct{ chroma.Lexer }

func (resourceLexer) Config() *chroma.Config { return &resourceConfig }

func (lexer resourceLexer) Tokenise(options *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	next, err := lexer.Lexer.Tokenise(options, value)
	if err != nil {
		return nil, err
	}
	return func() chroma.Token {
		token := next()
		if token.Type.InCategory(chroma.Name) || token.Type.InCategory(chroma.Keyword) {
			token.Type = chroma.Text
			if _, found := slices.BinarySearch(resourceKeywords, token.Value); found {
				token.Type = chroma.Keyword
			}
		}
		return token
	}, nil
}

var _ = lexers.Register(resourceLexer{Lexer: lexers.Get("C++")})

// ResourceHighlighter preserves C-style block comment context for one document.
type ResourceHighlighter struct{ *lineHighlighter[byte] }

func NewResourceHighlighter(shared *Highlighter) *ResourceHighlighter {
	h := newLineHighlighter(scanResourceLine)
	h.paint = func(_ string, index int, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
		if len(line) > components.CodeEditorMaxHighlightedLine {
			return dst
		}
		state, ok := h.stateAt(index)
		if !ok {
			return dst
		}
		prefix := ""
		if state == '*' {
			prefix = "/* "
		}
		return shared.highlight(".rc", line, prefix, dst)
	}
	return &ResourceHighlighter{h}
}

func scanResourceLine(line string, state byte, _ func(int, int, chroma.TokenType)) byte {
	// A physical line ending in backslash continues a quoted or // token.
	continued := strings.HasSuffix(line, `\`)
	if state == '/' {
		if continued {
			return '/'
		}
		return 0
	}
	for len(line) > 0 {
		switch state {
		case '*':
			end := strings.Index(line, "*/")
			if end < 0 {
				return '*'
			}
			line, state = line[end+2:], 0
		case '\'', '"':
			b := line[0]
			line = line[1:]
			if b == state {
				state = 0
			} else if b == '\\' && len(line) > 0 {
				line = line[1:]
			}
		default:
			start := strings.IndexAny(line, `/'"`)
			if start < 0 {
				return 0
			}
			line = line[start:]
			switch {
			case strings.HasPrefix(line, "//"):
				if continued {
					return '/'
				}
				return 0
			case strings.HasPrefix(line, "/*"):
				line, state = line[2:], '*'
			default:
				if line[0] != '/' {
					state = line[0]
				}
				line = line[1:]
			}
		}
	}
	if state == '*' || continued {
		return state
	}
	return 0
}
