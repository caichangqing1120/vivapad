package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/vivaui/viva/components"
)

// GoHighlighter keeps the multiline token state at the start of each line.
type GoHighlighter struct{ *lineHighlighter[byte] }

func NewGoHighlighter(shared *Highlighter) *GoHighlighter {
	h := newLineHighlighter(scanGoLine)
	h.paint = func(_ string, index int, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
		if len(line) > components.CodeEditorMaxHighlightedLine {
			return dst
		}
		state, ok := h.stateAt(index)
		if !ok {
			return dst
		}
		prefix, suffix := "", ""
		switch state {
		case '*':
			prefix = "/* "
		case '`':
			prefix = "`"
		}
		// The Go lexer needs complete tokens, even if the current editor line
		// only has a fragment. The renderer clips synthetic bytes from the runs.
		switch scanGoLine(line, state, nil) {
		case '*':
			suffix = " */"
		case '`':
			suffix = "`"
		}
		return shared.highlightContext(".go", line, prefix, suffix, dst)
	}
	return &GoHighlighter{h}
}

func scanGoLine(line string, state byte, _ func(int, int, chroma.TokenType)) byte {
	for len(line) > 0 {
		switch state {
		case '*':
			end := strings.Index(line, "*/")
			if end < 0 {
				return state
			}
			line, state = line[end+2:], 0
		case '`':
			end := strings.IndexByte(line, '`')
			if end < 0 {
				return state
			}
			line, state = line[end+1:], 0
		default:
			start := strings.IndexAny(line, "/'\"`")
			if start < 0 {
				return 0
			}
			line = line[start:]
			switch {
			case strings.HasPrefix(line, "//"):
				return 0
			case strings.HasPrefix(line, "/*"):
				line, state = line[2:], '*'
			case line[0] == '`':
				line, state = line[1:], '`'
			case line[0] == '\'' || line[0] == '"':
				quote := line[0]
				line = line[1:]
				for len(line) > 0 {
					b := line[0]
					line = line[1:]
					if b == quote {
						break
					}
					if b == '\\' && len(line) > 0 {
						line = line[1:]
					}
				}
			default:
				line = line[1:]
			}
		}
	}
	return state
}
