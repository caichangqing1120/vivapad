package syntax

import (
	"slices"
	"sort"
	"strings"

	"github.com/vivaui/viva/components"
)

type json5Line struct {
	start int
	state byte
}

// JSON5Highlighter keeps line-start context for one document. Chroma still
// paints the tokens; the index only tracks delimiters that cross a newline.
type JSON5Highlighter struct {
	shared *Highlighter
	value  string
	lines  []json5Line
	paint  func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun
}

func NewJSON5Highlighter(shared *Highlighter) *JSON5Highlighter {
	h := &JSON5Highlighter{shared: shared, lines: []json5Line{{}}}
	h.paint = h.highlightLine
	return h
}

// ForText binds the current immutable document snapshot to the indexed painter.
func (h *JSON5Highlighter) ForText(value string) func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun {
	if value != h.value {
		limit := min(h.lines[len(h.lines)-1].start, len(value))
		// A line's state stays valid when all bytes before its start match.
		if h.value[:limit] != value[:limit] {
			for i := 0; i < limit; i++ {
				if h.value[i] != value[i] {
					limit = i
					break
				}
			}
		}
		h.lines = h.lines[:sort.Search(len(h.lines), func(i int) bool { return h.lines[i].start > limit })]
		// A large deletion can leave an open document holding its peak index.
		// Reuse the allocation for ordinary edits that only invalidate a suffix.
		if len(value) < len(h.value)/2 && cap(h.lines) >= 4096 && len(h.lines) < cap(h.lines)/4 {
			h.lines = slices.Clone(h.lines)
		}
		h.value = value
	}
	return h.paint
}

func (h *JSON5Highlighter) highlightLine(_ string, index int, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	if index < 0 || len(line) > components.CodeEditorMaxHighlightedLine {
		return dst
	}
	for index >= len(h.lines) {
		prev := h.lines[len(h.lines)-1]
		remaining := h.value[prev.start:]
		end := strings.IndexByte(remaining, '\n')
		if end < 0 {
			return dst
		}
		h.lines = append(h.lines, json5Line{prev.start + end + 1, json5NextLineState(remaining[:end], prev.state)})
	}
	// A synthetic opener lets Chroma color the continuation and the regular
	// tokens after its closer. The highlighter removes it from drawn offsets.
	prefix := ""
	switch h.lines[index].state {
	case '*':
		prefix = "/*"
	case '\'':
		prefix = "'"
	case '"':
		prefix = `"`
	}
	return h.shared.highlight(".json5", line, prefix, dst)
}

func json5NextLineState(line string, state byte) byte {
	for len(line) != 0 {
		switch state {
		case '*':
			end := strings.Index(line, "*/")
			if end < 0 {
				return state
			}
			line, state = line[end+2:], 0
		case '\'', '"':
			quote := line[0] == state
			if line[0] == '\\' {
				if len(line) == 1 {
					return state
				}
				line = line[1:]
			}
			line = line[1:]
			if quote {
				state = 0
			}
		default:
			i := strings.IndexAny(line, `/'"`)
			if i < 0 {
				return 0
			}
			line = line[i:]
			switch {
			case strings.HasPrefix(line, "//"):
				end := strings.IndexAny(line, "\r\u2028\u2029")
				if end < 0 {
					return 0
				}
				line = line[end:]
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
	if state == '*' {
		return state
	}
	return 0
}
