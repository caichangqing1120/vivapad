package syntax

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

type Theme uint8

const (
	ClassicLight Theme = iota
	ModernDark
	Monokai
)

var modernColors = themeColors(styles.Get("github-dark"), gfx.RGB24(0xaab5bf), gfx.RGB24(0xff9b91))
var monokaiColors = themeColors(styles.Get("monokai"), gfx.RGB24(0xb5ae94), gfx.RGB24(0xff85ad))

func themeColors(style *chroma.Style, comment, diagnostic gfx.Color) map[gfx.Color]gfx.Color {
	fromToken := func(kind chroma.TokenType) gfx.Color {
		color := style.Get(kind).Colour
		return gfx.RGB(color.Red(), color.Green(), color.Blue())
	}
	keyword, _ := tokenColor(chroma.Keyword)
	stringColor, _ := tokenColor(chroma.LiteralString)
	number, _ := tokenColor(chroma.LiteralNumber)
	function, _ := tokenColor(chroma.NameFunction)
	classicComment, _ := tokenColor(chroma.Comment)
	numberKind := chroma.LiteralNumber
	if style.Name == "github-dark" {
		numberKind = chroma.NameConstant
	}
	return map[gfx.Color]gfx.Color{
		keyword:               readableToken(fromToken(chroma.Keyword)),
		stringColor:           fromToken(chroma.LiteralString),
		number:                readableToken(fromToken(numberKind)),
		function:              fromToken(chroma.NameFunction),
		classicComment:        comment,
		gfx.RGB(94, 118, 126): comment,
		gfx.RGB(31, 111, 124): readableToken(fromToken(chroma.NameTag)),
		gfx.RGB(170, 85, 0):   fromToken(chroma.LiteralString),
		gfx.RGB(177, 39, 58):  diagnostic,
		gfx.RGB(39, 112, 65):  fromToken(chroma.GenericInserted),
	}
}

func readableToken(color gfx.Color) gfx.Color {
	if color == gfx.RGB24(0xff7b72) || color == gfx.RGB24(0xae81ff) || color == gfx.RGB24(0xf92672) {
		return gfx.RGBA(255, 255, 255, 0.24).Over(color)
	}
	return color
}

// Recolor changes only caller-owned runs, leaving lexical caches and print colors intact.
func (theme Theme) Recolor(runs []components.CodeEditorRun) []components.CodeEditorRun {
	var colors map[gfx.Color]gfx.Color
	switch theme {
	case ModernDark:
		colors = modernColors
	case Monokai:
		colors = monokaiColors
	default:
		return runs
	}
	for index := range runs {
		if color, ok := colors[runs[index].Color]; ok {
			runs[index].Color = color
		}
	}
	return runs
}

func (theme Theme) Apply(mode string, snapshot *components.CodeEditorSnapshot) {
	if theme == ClassicLight || mode == ".ans" {
		return
	}
	if highlight := snapshot.HighlightLine; highlight != nil {
		snapshot.HighlightLine = func(mode string, line int, value string, dst []components.CodeEditorRun) []components.CodeEditorRun {
			runs := highlight(mode, line, value, dst)
			theme.Recolor(runs[len(dst):])
			return runs
		}
	} else if highlight := snapshot.Highlight; highlight != nil {
		snapshot.Highlight = func(mode, value string, dst []components.CodeEditorRun) []components.CodeEditorRun {
			runs := highlight(mode, value, dst)
			theme.Recolor(runs[len(dst):])
			return runs
		}
	}
}
