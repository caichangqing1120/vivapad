package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/syntax"
)

const udlManagerCommand = "User Defined Language..."
const udlLabelPrefix = "UDL: "

var (
	udlSaveID = tree.ID(page, "udl-save")
	udlNewID  = tree.ID(page, "udl-new")
	udlPrevID = tree.ID(page, "udl-previous")
	udlNextID = tree.ID(page, "udl-next")
	udlDropID = tree.ID(page, "udl-delete")
)

type udlHighlightCache struct {
	value, mode string
	highlight   func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun
}

func (state *model) udlForMode(mode string) *syntax.UDL {
	if !strings.HasPrefix(mode, "udl:") {
		return nil
	}
	for index := range state.udls {
		if state.udls[index].Mode() == mode {
			return &state.udls[index]
		}
	}
	return nil
}

func (state *model) languageForCommand(command string) (string, bool) {
	if name, custom := strings.CutPrefix(command, udlLabelPrefix); custom {
		for _, definition := range state.udls {
			if definition.Name == name {
				return definition.Mode(), true
			}
		}
	}
	return languageExtension(command)
}

func (state *model) openUDLManager(index int) {
	var definition syntax.UDL
	if index >= 0 && index < len(state.udls) {
		definition = state.udls[index]
	} else {
		index = -1
	}
	state.openM5Dialog(udlManagerCommand, index, definition.Name, strings.Join(definition.Extensions, ", "),
		strings.Join(definition.Keywords, " "), definition.LineComment, definition.BlockStart, definition.BlockEnd, strings.Join(definition.Quotes, " "))
}

func (state *model) setUDLDefinitions(definitions []syntax.UDL, previous, next string) error {
	if err := syntax.SaveUDLs(state.udlPath, definitions); err != nil {
		return err
	}
	loaded, err := syntax.LoadUDLs(state.udlPath)
	if err != nil {
		return err
	}
	state.udls = loaded
	clear(state.udlHighlights)
	for _, doc := range state.space.Documents {
		if previous != "" && state.languages[doc] == previous {
			if next != "" {
				state.languages[doc] = next
			} else {
				delete(state.languages, doc)
			}
		}
		state.configureCode(doc)
	}
	state.invalidator.Invalidate()
	return nil
}

func (state *model) paintUDLManager(canvas *gfx.Canvas, bounds gfx.Rect, dialog *m5Dialog) {
	palette := state.palette()
	left, top, width := bounds.X, bounds.Y, bounds.Width
	smallLabel(canvas, "User Defined Language", gfx.Rect{X: left, Y: top, Width: width, Height: 24}, palette.text, 15)
	row := gfx.Rect{X: left, Y: top + 31, Width: 30, Height: 26}
	if palette.m5Button(canvas, udlPrevID, row, "<", len(state.udls) == 0 || dialog.index == 0) {
		index := dialog.index - 1
		if dialog.index < 0 {
			index = len(state.udls) - 1
		}
		state.openUDLManager(index)
		return
	}
	row.X += 34
	if palette.m5Button(canvas, udlNextID, row, ">", dialog.index >= len(state.udls)-1) {
		state.openUDLManager(dialog.index + 1)
		return
	}
	row.X, row.Width = left+68, 52
	if palette.m5Button(canvas, udlNewID, row, "New", dialog.index < 0) {
		state.openUDLManager(-1)
		return
	}
	name := "New language"
	if dialog.index >= 0 {
		name = fmt.Sprintf("%d / %d    %s", dialog.index+1, len(state.udls), state.udls[dialog.index].Name)
	}
	smallLabel(canvas, canvas.FitLine(name, 12, gfx.WeightRegular, width-134), gfx.Rect{X: left + 130, Y: row.Y, Width: width - 134, Height: row.Height}, palette.muted, 12)
	labels := [...]string{"Name", "File extensions", "Keywords", "Line comment", "Block start", "Block end", "String quotes"}
	placeholders := [...]string{"My language", "foo, bar", "if else return", "//", "/*", "*/", `" '`}
	for index, label := range labels {
		columnWidth := (width - 14) / 2
		x, y := left+float32(index%2)*(columnWidth+14), top+63+float32(index/2)*46
		smallLabel(canvas, label, gfx.Rect{X: x, Y: y, Width: columnWidth, Height: 16}, palette.muted, 11)
		state.paintM5Field(canvas, gfx.Rect{X: x, Y: y + 17, Width: columnWidth, Height: 25}, index, label, placeholders[index], false)
	}
	smallLabel(canvas, canvas.FitLine(dialog.message, 11, gfx.WeightRegular, width), gfx.Rect{X: left, Y: top + 248, Width: width, Height: 20}, palette.diff[0], 11)
	bottom := top + bounds.Height - 29
	if palette.m5Button(canvas, udlDropID, gfx.Rect{X: left, Y: bottom, Width: 74, Height: 29}, "Delete", dialog.index < 0) {
		definitions := slices.Delete(slices.Clone(state.udls), dialog.index, dialog.index+1)
		if err := state.setUDLDefinitions(definitions, state.udls[dialog.index].Mode(), ""); err != nil {
			dialog.message = err.Error()
		} else {
			state.openUDLManager(min(dialog.index, len(definitions)-1))
		}
		return
	}
	if palette.m5Button(canvas, udlSaveID, gfx.Rect{X: left + width - 156, Y: bottom, Width: 74, Height: 29}, "Save", false) {
		field := func(index int) string { return strings.TrimSpace(dialog.fields[index].String()) }
		definition := syntax.UDL{Name: field(0), Extensions: strings.FieldsFunc(field(1), func(value rune) bool { return unicode.IsSpace(value) || value == ',' }),
			Keywords: strings.Fields(field(2)), LineComment: field(3), BlockStart: field(4), BlockEnd: field(5), Quotes: strings.Fields(field(6))}
		definitions := slices.Clone(state.udls)
		previous := ""
		if dialog.index < 0 {
			definitions = append(definitions, definition)
		} else {
			previous = definitions[dialog.index].Mode()
			definitions[dialog.index] = definition
		}
		if err := state.setUDLDefinitions(definitions, previous, definition.Mode()); err != nil {
			dialog.message = err.Error()
		} else {
			state.notice = "User language saved"
			state.closeM5Dialog()
		}
	}
}

func (state *model) applyUDLHighlight(doc *document.Document, snapshot *components.CodeEditorSnapshot) {
	definition := state.udlForMode(snapshot.Language)
	if definition == nil {
		delete(state.udlHighlights, doc)
		return
	}
	cache := state.udlHighlights[doc]
	if cache.highlight == nil || cache.value != snapshot.Value || cache.mode != snapshot.Language {
		cache = udlHighlightCache{value: snapshot.Value, mode: snapshot.Language, highlight: definition.Highlighter(snapshot.Value)}
		state.udlHighlights[doc] = cache
	}
	snapshot.HighlightLine = cache.highlight
}
