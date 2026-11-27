package main

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/websearch"
)

const webSearchCommand = "Search on Internet"
const webSearchSettingsCommand = "Change Search Engine..."

var webSearchEngines = websearch.Engines()

func (state *model) searchActiveEditorOnInternet() {
	pane := state.activePane
	doc := state.paneDocument(pane)
	editor := doc.Editor
	if editor.CompositionActive() {
		state.notice = "Finish input method composition before searching"
		return
	}
	if editor.SelectionCount() != 1 {
		state.notice = "Select a single text range to search"
		return
	}
	value := editor.String()
	if rect := state.rectangles[pane]; rect.active && rect.doc == doc && rect.value == value {
		state.notice = "Select a single text range to search"
		return
	}
	if multi := state.multiSelections[pane]; multi.active && multi.doc == doc && multi.value == value && len(multi.ranges) > 1 {
		state.notice = "Select a single text range to search"
		return
	}
	selection := editor.Selection()
	query := value[editor.ByteOffset(selection.Start):editor.ByteOffset(selection.End)]
	if selection.Start == selection.End {
		query = webSearchWordAt(value, editor.ByteOffset(editor.Caret()))
	}
	target, err := state.webSearch.URL(query)
	if err == nil {
		err = app.OpenURL(target)
		if err != nil {
			state.notice = "Could not open the search in the browser"
			return
		}
	} else {
		state.notice = err.Error()
	}
}

func webSearchWordAt(value string, caret int) string {
	isWord := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_' }
	start, end := caret, caret
	count := 0
	for start > 0 && count <= 1024 {
		r, n := utf8.DecodeLastRuneInString(value[:start])
		if !isWord(r) {
			break
		}
		start -= n
		count++
	}
	for end < len(value) && count <= 1024 {
		r, n := utf8.DecodeRuneInString(value[end:])
		if !isWord(r) {
			break
		}
		end += n
		count++
	}
	return value[start:end]
}

func (state *model) openWebSearchSettings() {
	state.webSearchOpen, state.webSearchChoice = true, state.webSearch.EngineID()
	state.webSearchDraft = text.NewEditor(state.webSearch.Template)
	state.webSearchError = ""
}

func (state *model) closeWebSearchSettings() {
	state.webSearchOpen, state.webSearchDraft = false, nil
	state.webSearchError = ""
	state.requestEditorCommandFocus()
}

func (state *model) saveWebSearchSettings() {
	next := websearch.Config{Engine: state.webSearchChoice}
	if next.Engine == "custom" {
		next.Template = strings.TrimSpace(state.webSearchDraft.String())
	}
	if err := next.Validate(); err != nil {
		state.webSearchError = err.Error()
		state.webSearchErrorTemplate = state.webSearchDraft.String()
		return
	}
	previous := state.webSearch
	state.webSearch = next
	if err := state.savePreferences(); err != nil {
		state.webSearch = previous
		state.webSearchError = "Could not save the search engine"
		state.webSearchErrorTemplate = state.webSearchDraft.String()
		return
	}
	state.closeWebSearchSettings()
}

func (state *model) paintWebSearchSettings(canvas *gfx.Canvas, size gfx.Size) {
	palette := state.palette()
	const title = "Search Engine"
	width, height := min(float32(530), size.Width-24), float32(246)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.PushFocusScope(webSearchDialogID)
	canvas.TrapFocus(webSearchDialogID)
	canvas.SetInitialFocus(webSearchEngineScope.At(0))
	canvas.SetEscapeTarget(webSearchCancelID)
	canvas.PushLayer(4)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.RoundedBorder(panel, 4, 1, palette.border, palette.popup)
	canvas.HitWithOptions(webSearchDialogID, panel, title, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	smallLabel(canvas, title, gfx.Rect{X: x + 18, Y: y + 10, Width: width - 36, Height: 30}, palette.text, 15)
	choiceWidth := (width - 48) / 3
	for i, engine := range webSearchEngines {
		id := webSearchEngineScope.At(i)
		bounds := gfx.Rect{X: x + 18 + float32(i%3)*(choiceWidth+6), Y: y + 50 + float32(i/3)*32, Width: choiceWidth, Height: 26}
		checked := engine.ID == state.webSearchChoice
		border, fill := palette.border, palette.toolbar
		if checked || canvas.FocusVisible() == id {
			border = palette.accent
		}
		if checked {
			fill = palette.hover
		}
		canvas.RoundedBorder(bounds, 4, 1, border, fill)
		canvas.LabelCenter(canvas.FitLine(engine.Label, 11, gfx.WeightRegular, bounds.Width-8), bounds, gfx.TextStyle{Size: 11, Color: palette.text})
		canvas.HitWithOptions(id, bounds, engine.Label, gfx.HitOptions{Role: gfx.RoleButton, Checked: checked})
		if canvas.Clicked(id) {
			state.webSearchChoice, state.webSearchError = engine.ID, ""
			if engine.ID == "custom" {
				canvas.RequestFocus(webSearchTemplateID)
			}
		}
	}
	if state.webSearchChoice == "custom" {
		input := gfx.Rect{X: x + 18, Y: y + 120, Width: width - 36, Height: 28}
		value := state.webSearchDraft.String()
		if state.webSearchError != "" && value != state.webSearchErrorTemplate {
			state.webSearchError = ""
		}
		components.PaintInput(canvas, state.uiTheme(), input, components.InputSnapshot{
			ID: webSearchTemplateID, Label: "HTTPS search template", Value: value,
			Placeholder: "https://example.com/search?q=" + websearch.Placeholder, PlatformEditing: canvas.Focused() == webSearchTemplateID,
		})
	}
	errBounds := gfx.Rect{X: x + 18, Y: y + 154, Width: width - 36, Height: 26}
	smallLabel(canvas, canvas.FitLine(state.webSearchError, 11, gfx.WeightRegular, errBounds.Width), errBounds, palette.text, 11)
	for i, item := range []struct {
		id    gfx.ID
		label string
	}{{webSearchSaveID, "Save"}, {webSearchCancelID, "Cancel"}} {
		bounds := gfx.Rect{X: x + width - 194 + float32(i)*92, Y: y + height - 45, Width: 84, Height: 28}
		border := palette.border
		if canvas.FocusVisible() == item.id {
			border = palette.accent
		}
		canvas.RoundedBorder(bounds, 4, 1, border, palette.toolbar)
		canvas.LabelCenter(item.label, bounds, gfx.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, item.id, bounds, item.label) {
			if i == 0 {
				state.saveWebSearchSettings()
			} else {
				state.closeWebSearchSettings()
			}
		}
	}
	canvas.PopLayer()
	canvas.PopFocusScope()
}
