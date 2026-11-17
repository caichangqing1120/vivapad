package main

import (
	"strconv"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/session"
	"vivapad/internal/syntax"
)

type functionSymbolCache struct {
	value   string
	mode    string
	symbols []syntax.Symbol
	err     error
}

func (state *model) restoreFunctionDock(stored session.State) {
	state.functionListVisible = stored.FunctionListVisible
	state.documentListVisible = stored.DocumentListVisible
	state.activeLeftDock = stored.ActiveLeftDock
	state.activeLeftDock = state.activeDock()
}

func (state *model) activeDock() string {
	if state.documentListVisible && state.activeLeftDock == "documents" {
		return "documents"
	}
	if state.functionListVisible && state.activeLeftDock == "functions" {
		return "functions"
	}
	if state.folderVisible {
		return "workspace"
	}
	if state.functionListVisible {
		return "functions"
	}
	if state.documentListVisible {
		return "documents"
	}
	return ""
}

func (state *model) symbolsFor(doc *document.Document) functionSymbolCache {
	value, mode := doc.Editor.String(), state.languageFor(doc)
	if cached, ok := state.functionSymbols[doc]; ok && cached.value == value && cached.mode == mode {
		return cached
	}
	symbols, err := syntax.Symbols(mode, doc.Name, value)
	result := functionSymbolCache{value: value, mode: mode, symbols: symbols, err: err}
	state.functionSymbols[doc] = result
	return result
}

func (state *model) scrollFunctionList(event gfx.ScrollEvent) gfx.EventResult {
	step := 1
	if event.StepY(22) < 0 {
		step = -1
	}
	state.functionOffset = min(max(0, state.functionOffset+step), max(0, len(state.functionMatches)-state.functionPageRows))
	if len(state.functionMatches) != 0 {
		state.functionSelected = min(max(state.functionSelected, state.functionOffset), state.functionOffset+state.functionPageRows-1)
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (state *model) goToFunction(doc *document.Document, symbol syntax.Symbol) bool {
	cached := state.symbolsFor(doc)
	if cached.value != doc.Editor.String() || symbol.ByteOffset < 0 || symbol.ByteOffset >= len(cached.value) {
		state.notice = "Function List result is stale"
		return false
	}
	name := symbol.Name
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		name = name[dot+1:]
	}
	if !strings.HasPrefix(cached.value[symbol.ByteOffset:], name) {
		state.notice = "Function List result is stale"
		return false
	}
	doc.Editor.MoveTo(doc.Editor.RuneIndex(symbol.ByteOffset), false)
	state.codeEditor().Reveal(symbol.ByteOffset)
	state.invalidator.Invalidate()
	return true
}

func (state *model) functionListKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	switch event.Key {
	case gfx.KeyDown:
		if len(state.functionMatches) != 0 {
			state.functionSelected = min(len(state.functionMatches)-1, state.functionSelected+1)
		}
		return gfx.EventResult{Handled: true, Repaint: true}, true
	case gfx.KeyUp:
		if len(state.functionMatches) != 0 {
			state.functionSelected = max(0, state.functionSelected-1)
		}
		return gfx.EventResult{Handled: true, Repaint: true}, true
	case gfx.KeyEnter:
		if state.functionSelected >= 0 && state.functionSelected < len(state.functionMatches) {
			doc := state.space.Current()
			if state.splitEnabled {
				doc = state.paneDocument(state.activePane)
			}
			if state.functionPaintDoc != doc || state.functionPaintValue != doc.Editor.String() || state.functionPaintMode != state.languageFor(doc) {
				state.notice = "Function List result changed; select it again"
				return gfx.EventResult{Handled: true, Repaint: true}, true
			}
			cached := state.symbolsFor(doc)
			index := state.functionMatches[state.functionSelected]
			if index < 0 || index >= len(cached.symbols) {
				state.notice = "Function List result changed; select it again"
				return gfx.EventResult{Handled: true, Repaint: true}, true
			}
			state.goToFunction(doc, cached.symbols[index])
		}
		focus := editorID
		if state.splitEnabled && state.activePane == 1 {
			focus = rightEditorID
		}
		return gfx.EventResult{Handled: true, Repaint: true, Focus: focus}, true
	case gfx.KeyEscape:
		if state.functionFilter.String() != "" {
			state.functionFilter = text.NewEditor("")
			return gfx.EventResult{Handled: true, Repaint: true}, true
		}
		focus := editorID
		if state.splitEnabled && state.activePane == 1 {
			focus = rightEditorID
		}
		return gfx.EventResult{Handled: true, Focus: focus}, true
	}
	return gfx.EventResult{}, false
}

func (state *model) paintLeftDockTabs(canvas *gfx.Canvas, bounds gfx.Rect) {
	canvas.Fill(bounds, palette.tabs)
	canvas.Fill(gfx.Rect{X: bounds.Width - 1, Y: bounds.Y, Width: 1, Height: bounds.Height}, palette.border)
	canvas.Fill(gfx.Rect{Y: bounds.Y + bounds.Height - 1, Width: bounds.Width, Height: 1}, palette.border)
	var tabs [3]struct {
		id    gfx.ID
		label string
		mode  string
	}
	count := 0
	if state.documentListVisible {
		tabs[count] = struct {
			id          gfx.ID
			label, mode string
		}{leftDockDocumentsID, "Documents", "documents"}
		count++
	}
	if state.folderVisible {
		tabs[count] = struct {
			id          gfx.ID
			label, mode string
		}{leftDockWorkspaceID, "Workspace", "workspace"}
		count++
	}
	if state.functionListVisible {
		tabs[count] = struct {
			id          gfx.ID
			label, mode string
		}{leftDockFunctionsID, "Functions", "functions"}
		count++
	}
	for index, tab := range tabs[:count] {
		item := gfx.Rect{X: float32(index) * bounds.Width / float32(count), Y: bounds.Y, Width: bounds.Width / float32(count), Height: bounds.Height - 1}
		selected := state.activeDock() == tab.mode
		if selected {
			canvas.Fill(item, palette.white)
			canvas.Fill(gfx.Rect{X: item.X, Y: item.Y + item.Height - 2, Width: item.Width, Height: 2}, palette.blue)
		}
		label := tab.label
		if item.Width < 75 {
			switch tab.mode {
			case "documents":
				label = "Files"
			case "workspace":
				label = "Folder"
			case "functions":
				label = "Funcs"
			}
		}
		label = canvas.FitLine(label, 11, gfx.WeightRegular, item.Width-8)
		canvas.LabelCenter(label, item, gfx.TextStyle{Size: 11, Color: palette.text})
		canvas.HitWithOptions(tab.id, item, tab.label, gfx.HitOptions{Role: gfx.RoleTab, Selected: selected})
		if canvas.Clicked(tab.id) {
			state.activeLeftDock = tab.mode
			state.functionFilterFocus = tab.mode == "functions"
		}
	}
}

func (state *model) paintFunctionPanel(canvas *gfx.Canvas, bounds gfx.Rect) {
	canvas.Fill(bounds, gfx.RGB(250, 250, 250))
	canvas.Fill(gfx.Rect{X: bounds.Width - 1, Y: bounds.Y, Width: 1, Height: bounds.Height}, palette.border)
	canvas.Fill(gfx.Rect{X: 0, Y: bounds.Y + 32, Width: bounds.Width, Height: 1}, palette.border)
	smallLabel(canvas, "Function List", gfx.Rect{X: 9, Y: bounds.Y + 3, Width: bounds.Width - 39, Height: 27}, palette.text, 11)
	closeBounds := gfx.Rect{X: bounds.Width - 26, Y: bounds.Y + 7, Width: 18, Height: 18}
	canvas.DrawIcon(closeIcon, closeBounds.Inset(gfx.All(3)), palette.muted)
	if hitButton(canvas, functionCloseID, closeBounds, "Close Function List") {
		state.functionListVisible = false
		if state.folderVisible {
			state.activeLeftDock = "workspace"
		} else if state.documentListVisible {
			state.activeLeftDock = "documents"
		} else {
			state.activeLeftDock = ""
		}
	}
	theme := gfx.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 26, 11, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	components.PaintInput(canvas, theme, gfx.Rect{X: 8, Y: bounds.Y + 39, Width: bounds.Width - 17, Height: 26}, components.InputSnapshot{
		ID: functionFilterID, Label: "Filter functions", Placeholder: "Filter", Value: state.functionFilter.String(), PlatformEditing: canvas.Focused() == functionFilterID,
	})
	if state.functionFilterFocus {
		canvas.RequestFocus(functionFilterID)
		state.functionFilterFocus = false
	}
	rowArea := gfx.Rect{X: 1, Y: bounds.Y + 72, Width: bounds.Width - 2, Height: max(0, bounds.Height-72)}
	canvas.HitWithOptions(functionPanelID, rowArea, "Function List results", gfx.HitOptions{Role: gfx.RoleGroup})
	doc := state.space.Current()
	if state.splitEnabled {
		doc = state.paneDocument(state.activePane)
	}
	cached := state.symbolsFor(doc)
	query := strings.ToLower(strings.TrimSpace(state.functionFilter.String()))
	stale := state.functionPaintDoc != nil && (state.functionPaintDoc != doc || state.functionPaintValue != cached.value || state.functionPaintMode != cached.mode || state.functionLastQuery != query)
	if state.functionPaintDoc != doc || state.functionPaintMode != cached.mode || state.functionLastQuery != query {
		state.functionSelected, state.functionOffset = 0, 0
	}
	state.functionMatches = state.functionMatches[:0]
	for index, symbol := range cached.symbols {
		if query == "" || strings.Contains(strings.ToLower(symbol.Name), query) {
			state.functionMatches = append(state.functionMatches, index)
		}
	}
	state.functionPageRows = max(1, int(rowArea.Height/23))
	if len(state.functionMatches) == 0 {
		state.functionSelected, state.functionOffset = -1, 0
	} else {
		state.functionSelected = min(max(0, state.functionSelected), len(state.functionMatches)-1)
		if state.functionSelected < state.functionOffset {
			state.functionOffset = state.functionSelected
		} else if state.functionSelected >= state.functionOffset+state.functionPageRows {
			state.functionOffset = state.functionSelected - state.functionPageRows + 1
		}
		state.functionOffset = min(max(0, state.functionOffset), max(0, len(state.functionMatches)-state.functionPageRows))
	}
	canvas.Clip(rowArea)
	selected := -1
	for position := state.functionOffset; position < len(state.functionMatches) && position-state.functionOffset < state.functionPageRows; position++ {
		symbol := cached.symbols[state.functionMatches[position]]
		item := gfx.Rect{X: 4, Y: rowArea.Y + float32(position-state.functionOffset)*23, Width: rowArea.Width - 7, Height: 23}
		id := functionSymbolScope.At(position)
		if position == state.functionSelected || canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(symbol.Name, 11, gfx.WeightRegular, item.Width-17)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 10, Y: item.Y, Width: item.Width - 17, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, symbol.Name, gfx.HitOptions{Role: gfx.RoleButton, Hint: symbol.Kind + " on line " + strconv.Itoa(symbol.Line)})
		if canvas.Clicked(id) {
			selected = position
		}
	}
	if len(state.functionMatches) == 0 {
		message := "No symbols"
		if query != "" {
			message = "No matches"
		} else if cached.err != nil {
			message = "Symbols unavailable"
		}
		smallLabel(canvas, message, gfx.Rect{X: 12, Y: rowArea.Y + 8, Width: rowArea.Width - 24, Height: 24}, palette.muted, 11)
	}
	canvas.Unclip()
	state.functionPaintDoc, state.functionPaintValue, state.functionPaintMode, state.functionLastQuery = doc, cached.value, cached.mode, query
	if selected >= 0 && !stale {
		state.functionSelected = selected
		state.goToFunction(doc, cached.symbols[state.functionMatches[selected]])
	}
}
