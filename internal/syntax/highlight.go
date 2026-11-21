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
	prefix   string
}

type Highlighter struct {
	lexers map[string]chroma.Lexer
	lines  map[cacheKey][]components.CodeEditorRun
}

func (highlighter *Highlighter) Highlight(language, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	return highlighter.highlight(language, line, "", dst)
}

func (highlighter *Highlighter) highlight(language, line, prefix string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	if language == "" || len(line) > components.CodeEditorMaxHighlightedLine {
		return dst
	}
	key := cacheKey{language: language, line: line, prefix: prefix}
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
	source := prefix + line
	// Chroma's TeX comment rule consumes the newline, which the editor omits.
	if lexer.Config().Name == "TeX" && strings.Contains(line, "%") && !strings.HasSuffix(line, "\n") {
		source += "\n"
	}
	iterator, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root"}, source)
	if err != nil {
		return dst
	}
	var runs []components.CodeEditorRun
	_, record := lexer.(*hexRecordLexer)
	_, mmixal := lexer.(*mmixalLexer)
	_, errorList := lexer.(*errorListLexer)
	_, kix := lexer.(*kixLexer)
	_, csound := lexer.(*csoundLexer)
	_, avs := lexer.(*avsLexer)
	_, oscript := lexer.(*oscriptLexer)
	_, gui4cli := lexer.(*gui4CliLexer)
	_, nncrontab := lexer.(*nncrontabLexer)
	_, txt2tags := lexer.(*txt2tagsLexer)
	_, hollywood := lexer.(*hollywoodLexer)
	_, visualProlog := lexer.(*visualPrologLexer)
	_, escript := lexer.(*escriptLexer)
	_, baan := lexer.(*baanLexer)
	for offset := 0; ; {
		token := iterator()
		if token == chroma.EOF {
			break
		}
		start := max(0, offset-len(prefix))
		end := min(offset+len(token.Value)-len(prefix), len(line))
		color, ok := tokenColor(token.Type)
		if (record || mmixal) && token.Type == chroma.Error {
			color, ok = gfx.RGB(177, 39, 58), true
		} else if mmixal {
			switch token.Type {
			case chroma.NameLabel, chroma.NameConstant:
				color, ok = tokenColor(chroma.NameClass)
			case chroma.NameVariable:
				color, ok = tokenColor(chroma.LiteralNumber)
			}
		} else if errorList {
			color, ok = errorListColor(token.Type)
		} else if kix {
			color, ok = kixTokenColor(token.Type)
		} else if csound {
			color, ok = csoundTokenColor(token.Type)
		} else if avs {
			color, ok = avsTokenColor(token.Type)
		} else if oscript {
			color, ok = oscriptTokenColor(token.Type)
		} else if gui4cli {
			color, ok = gui4CliTokenColor(token.Type)
		} else if nncrontab {
			color, ok = nncrontabTokenColor(token.Type)
		} else if txt2tags {
			color, ok = txt2tagsTokenColor(token.Type)
		} else if hollywood {
			color, ok = hollywoodTokenColor(token.Type)
		} else if visualProlog {
			color, ok = visualPrologTokenColor(token.Type)
		} else if escript {
			color, ok = escriptTokenColor(token.Type)
		} else if baan {
			color, ok = baanTokenColor(token.Type)
		}
		if ok && end > start {
			runs = append(runs, components.CodeEditorRun{Start: start, End: end, Color: color})
		}
		offset += len(token.Value)
		if offset >= len(line)+len(prefix) {
			break
		}
	}
	if len(highlighter.lines) >= 1024 {
		clear(highlighter.lines)
	}
	if highlighter.lines == nil {
		highlighter.lines = make(map[cacheKey][]components.CodeEditorRun)
	}
	key.line = strings.Clone(line)
	highlighter.lines[key] = runs
	return append(dst, runs...)
}

func csoundTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.NameVariable:
		return tokenColor(chroma.LiteralNumber)
	case chroma.NameVariableGlobal:
		return gfx.RGB(113, 76, 144), true
	default:
		return tokenColor(kind)
	}
}

func kixTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.NameVariable, chroma.NameConstant:
		return tokenColor(chroma.LiteralNumber)
	default:
		return tokenColor(kind)
	}
}

func errorListColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.GenericError, chroma.GenericDeleted:
		return gfx.RGB(177, 39, 58), true
	case chroma.GenericInserted:
		return gfx.RGB(39, 112, 65), true
	case chroma.GenericTraceback:
		return tokenColor(chroma.NameClass)
	case chroma.GenericHeading, chroma.GenericPrompt:
		return tokenColor(chroma.Keyword)
	case chroma.GenericEmph:
		return tokenColor(chroma.Comment)
	default:
		return gfx.Color{}, false
	}
}

func tokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch {
	case kind.InCategory(chroma.Comment):
		return gfx.RGB(92, 125, 87), true
	case kind.InCategory(chroma.Keyword), kind.InSubCategory(chroma.NameTag):
		return gfx.RGB(49, 92, 167), true
	case kind.InSubCategory(chroma.LiteralNumber):
		return gfx.RGB(139, 75, 147), true
	case kind.InCategory(chroma.Literal):
		return gfx.RGB(172, 77, 64), true
	case kind.InSubCategory(chroma.NameFunction), kind.InSubCategory(chroma.NameClass):
		return gfx.RGB(113, 76, 144), true
	}
	return gfx.Color{}, false
}
