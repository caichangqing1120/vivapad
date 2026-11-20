package syntax

import (
	"slices"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

type syntaxLine[S any] struct {
	start int
	state S
}

type scanLine[S any] func(string, S, func(int, int, chroma.TokenType)) S

func tokeniseSyntaxLines[S any](value string, scan scanLine[S]) chroma.Iterator {
	var state S
	var tokens []chroma.Token
	for value != "" {
		end, next := syntaxLineBounds(value)
		line, cursor := value[:end], 0
		state = scan(line, state, func(start, end int, kind chroma.TokenType) {
			if cursor < start {
				tokens = append(tokens, chroma.Token{Type: chroma.Text, Value: line[cursor:start]})
			}
			tokens = append(tokens, chroma.Token{Type: kind, Value: line[start:end]})
			cursor = end
		})
		if cursor < next {
			tokens = append(tokens, chroma.Token{Type: chroma.Text, Value: value[cursor:next]})
		}
		value = value[next:]
	}
	return chroma.Literator(tokens...)
}

func syntaxLineBounds(value string) (end, next int) {
	end = strings.IndexAny(value, "\r\n")
	if end < 0 {
		return len(value), len(value)
	}
	next = end + 1
	if value[end] == '\r' && next < len(value) && value[next] == '\n' {
		next++
	}
	return end, next
}

// lineHighlighter indexes line-start state lazily and only styles visible lines.
type lineHighlighter[S any] struct {
	value string
	lines []syntaxLine[S]
	scan  scanLine[S]
	color func(chroma.TokenType) (gfx.Color, bool)
	paint func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun
}

func newLineHighlighter[S any](scan scanLine[S]) *lineHighlighter[S] {
	h := &lineHighlighter[S]{lines: []syntaxLine[S]{{}}, scan: scan, color: tokenColor}
	h.paint = h.highlightLine
	return h
}

func (h *lineHighlighter[S]) ForText(value string) func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun {
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

func (h *lineHighlighter[S]) highlightLine(_ string, index int, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
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
		state := h.scan(remaining[:end], prev.state, nil)
		h.lines = append(h.lines, syntaxLine[S]{prev.start + end + 1, state})
	}
	h.scan(line, h.lines[index].state, func(start, end int, kind chroma.TokenType) {
		if color, ok := h.color(kind); ok {
			dst = append(dst, components.CodeEditorRun{Start: start, End: end, Color: color})
		}
	})
	return dst
}
