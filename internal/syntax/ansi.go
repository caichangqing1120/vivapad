package syntax

import (
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

type ansiStyle struct{ fore, back int }

var ansiColors = [...]gfx.Color{
	gfx.RGB(0, 0, 0), gfx.RGB(205, 0, 0), gfx.RGB(0, 205, 0), gfx.RGB(170, 85, 0),
	gfx.RGB(0, 0, 255), gfx.RGB(205, 0, 205), gfx.RGB(0, 136, 136), gfx.RGB(229, 229, 229),
	gfx.RGB(127, 127, 127), gfx.RGB(255, 0, 0), gfx.RGB(0, 255, 0), gfx.RGB(255, 255, 0),
	gfx.RGB(92, 92, 255), gfx.RGB(255, 0, 255), gfx.RGB(0, 255, 255), gfx.RGB(255, 255, 255),
}

type ansiLexer struct{ config chroma.Config }

var _ = lexers.Register(&ansiLexer{config: chroma.Config{Name: "Escape Sequence (ANSI)", Aliases: []string{"escseq"}}})

func (l *ansiLexer) Config() *chroma.Config                         { return &l.config }
func (l *ansiLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *ansiLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*ansiLexer) AnalyseText(string) float32                       { return 0 }

func (*ansiLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, func(line string, _ struct{}, emit func(int, int, chroma.TokenType)) struct{} {
		scanANSI(line, func(start, end int, kind chroma.TokenType, _ ansiStyle) { emit(start, end, kind) })
		return struct{}{}
	}), nil
}

// scanANSI colors editable source. A new SGR specifies a fresh style, and
// neither its style nor an unfinished sequence carries into the next line.
func scanANSI(line string, emit func(int, int, chroma.TokenType, ansiStyle)) {
	style := ansiStyle{-1, -1}
	for at := 0; at < len(line); {
		index := strings.Index(line[at:], "\x1b[")
		if index < 0 {
			emit(at, len(line), chroma.Text, style)
			return
		}
		start := at + index
		if start > at {
			emit(at, start, chroma.Text, style)
		}
		end := start + 2
		for end < len(line) && line[end] >= ' ' && line[end] <= '?' {
			end++
		}
		kind := chroma.Error
		if end < len(line) && line[end] >= '@' && line[end] <= '~' {
			params := line[start+2 : end]
			switch line[end] {
			case 'm':
				if next, ok := ansiSGR(params); ok {
					style, kind = next, chroma.NameDecorator
				}
			case 'K':
				if params == "" || params == "0" || params == "1" || params == "2" {
					kind = chroma.NameDecorator
				}
			}
			end++
		}
		if kind == chroma.Error {
			style = ansiStyle{-1, -1}
		}
		emit(start, end, kind, ansiStyle{-1, -1})
		at = end
	}
}

func ansiSGR(params string) (ansiStyle, bool) {
	style := ansiStyle{-1, -1}
	for i := range len(params) {
		if ch := params[i]; ch != ';' && (ch < '0' || ch > '9') {
			return style, false
		}
	}
	bright := false
	for {
		field, rest, more := strings.Cut(params, ";")
		params = rest
		code, err := strconv.Atoi(field)
		if err == nil || field == "" {
			switch {
			case code == 0:
				style, bright = ansiStyle{-1, -1}, false
			case code == 1:
				bright = true
			case code == 22:
				bright = false
			case code >= 30 && code <= 37:
				style.fore = code - 30
			case code >= 40 && code <= 47:
				style.back = code - 40
			case code >= 90 && code <= 97:
				style.fore = code - 90 + 8
			case code >= 100 && code <= 107:
				style.back = code - 100 + 8
			case code == 39:
				style.fore = -1
			case code == 49:
				style.back = -1
			case code == 38 || code == 48 || code == 58:
				// Extended color channels are not separate basic SGR commands.
				mode, tail, remaining := strings.Cut(params, ";")
				params, more = tail, remaining
				count := 0
				switch mode {
				case "2":
					count = 3
				case "5":
					count = 1
				default:
					more = false
				}
				for j := 0; j < count && more; j++ {
					_, params, more = strings.Cut(params, ";")
				}
			}
		}
		if !more {
			break
		}
	}
	if bright && style.fore >= 0 && style.fore < 8 {
		style.fore += 8
	}
	return style, true
}

func highlightANSI(line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	scanANSI(line, func(start, end int, kind chroma.TokenType, style ansiStyle) {
		var color gfx.Color
		switch {
		case kind == chroma.Error:
			color = gfx.RGB(177, 39, 58)
		case kind == chroma.NameDecorator:
			color = gfx.RGB(113, 76, 144)
		case style.fore >= 0:
			color = ansiColors[style.fore]
		default:
			return
		}
		dst = append(dst, components.CodeEditorRun{Start: start, End: end, Color: color})
	})
	return dst
}

// ANSIBackground indexes only the basic background palette, reusing sorted
// spans until the source changes. The editor clips them to the visible rows.
type ANSIBackground struct {
	value  string
	spans  [len(ansiColors)][]text.Span
	layers []components.CodeEditorMarkLayer
}

func (background *ANSIBackground) AppendLayers(value string, dst []components.CodeEditorMarkLayer) []components.CodeEditorMarkLayer {
	if value != background.value {
		background.value = value
		for i := range background.spans {
			background.spans[i] = background.spans[i][:0]
		}
		if strings.Contains(value, "\x1b[") {
			for base := 0; base < len(value); {
				end, next := syntaxLineBounds(value[base:])
				if end <= components.CodeEditorMaxHighlightedLine {
					scanANSI(value[base:base+end], func(start, end int, _ chroma.TokenType, style ansiStyle) {
						if style.back >= 0 {
							spans := &background.spans[style.back]
							*spans = append(*spans, text.Span{Start: base + start, End: base + end})
						}
					})
				}
				base += next
			}
		}
		clear(background.layers)
		background.layers = background.layers[:0]
		for i, spans := range background.spans {
			if len(spans) < cap(spans)/4 && cap(spans) > 4096 {
				spans = slices.Clone(spans)
				if len(spans) == 0 {
					spans = nil
				}
				background.spans[i] = spans
			}
			if len(spans) != 0 {
				background.layers = append(background.layers, components.CodeEditorMarkLayer{Spans: spans, Color: ansiColors[i]})
			}
		}
	}
	return append(dst, background.layers...)
}
