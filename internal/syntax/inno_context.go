package syntax

import (
	"slices"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/vivaui/viva/components"
)

type innoLine struct {
	start int
	state innoState
}

// InnoHighlighter keeps the section and Pascal comment state at each indexed
// line without storing tokens for the entire document.
type InnoHighlighter struct {
	value string
	lines []innoLine
	paint func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun
}

func NewInnoHighlighter() *InnoHighlighter {
	h := &InnoHighlighter{lines: []innoLine{{}}}
	h.paint = h.highlightLine
	return h
}

func (h *InnoHighlighter) ForText(value string) func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun {
	if value != h.value {
		limit := min(len(value), h.lines[len(h.lines)-1].start)
		if h.value[:limit] != value[:limit] {
			for i := 0; i < limit; i++ {
				if h.value[i] != value[i] {
					limit = i
					break
				}
			}
		}
		h.lines = h.lines[:sort.Search(len(h.lines), func(i int) bool { return h.lines[i].start > limit })]
		if len(value) < len(h.value)/2 && cap(h.lines) >= 4096 && len(h.lines) < cap(h.lines)/4 {
			h.lines = slices.Clone(h.lines)
		}
		h.value = value
	}
	return h.paint
}

func (h *InnoHighlighter) highlightLine(_ string, index int, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
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
		h.lines = append(h.lines, innoLine{prev.start + end + 1, scanInnoLine(remaining[:end], prev.state, nil)})
	}
	scanInnoLine(line, h.lines[index].state, func(start, end int, kind chroma.TokenType) {
		if color, ok := tokenColor(kind); ok {
			dst = append(dst, components.CodeEditorRun{Start: start, End: end, Color: color})
		}
	})
	return dst
}
