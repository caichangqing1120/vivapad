package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

type cacheKey struct {
	language string
	line     string
}

type Highlighter struct {
	lexers map[string]chroma.Lexer
	lines  map[cacheKey][]components.CodeEditorRun
}

func (highlighter *Highlighter) Highlight(language, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	if language == "" || len(line) > components.CodeEditorMaxHighlightedLine {
		return dst
	}
	key := cacheKey{language: language, line: line}
	if cached, ok := highlighter.lines[key]; ok {
		return append(dst, cached...)
	}
	lexer, known := highlighter.lexers[language]
	if !known {
		if spec, ok := LanguageByMode(language); ok {
			lexer = lexers.Get(spec.Lexer)
		} else {
			lexer = lexers.Match("file" + language)
		}
		if highlighter.lexers == nil {
			highlighter.lexers = make(map[string]chroma.Lexer)
		}
		highlighter.lexers[language] = lexer
	}
	if lexer == nil {
		return dst
	}
	iterator, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root"}, line)
	if err != nil {
		return dst
	}
	var runs []components.CodeEditorRun
	for offset := 0; ; {
		token := iterator()
		if token == chroma.EOF {
			break
		}
		end := min(offset+len(token.Value), len(line))
		if color, ok := tokenColor(token.Type); ok && end > offset {
			runs = append(runs, components.CodeEditorRun{Start: offset, End: end, Color: color})
		}
		offset += len(token.Value)
		if offset >= len(line) {
			break
		}
	}
	if len(highlighter.lines) >= 1024 {
		clear(highlighter.lines)
	}
	if highlighter.lines == nil {
		highlighter.lines = make(map[cacheKey][]components.CodeEditorRun)
	}
	highlighter.lines[cacheKey{language: language, line: strings.Clone(line)}] = runs
	return append(dst, runs...)
}

func tokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch {
	case kind.InCategory(chroma.Comment):
		return gfx.RGB(92, 125, 87), true
	case kind.InCategory(chroma.Keyword), kind.InSubCategory(chroma.NameTag):
		return gfx.RGB(49, 92, 167), true
	case kind.InCategory(chroma.LiteralString):
		return gfx.RGB(172, 77, 64), true
	case kind.InCategory(chroma.LiteralNumber):
		return gfx.RGB(139, 75, 147), true
	case kind.InSubCategory(chroma.NameFunction), kind.InSubCategory(chroma.NameClass):
		return gfx.RGB(113, 76, 144), true
	}
	return gfx.Color{}, false
}
