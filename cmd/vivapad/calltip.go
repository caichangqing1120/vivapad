package main

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/completion"
	"vivapad/internal/document"
	"vivapad/internal/syntax"
)

const (
	parameterHintCommand = "Function Parameters Hint"
	previousHintCommand  = "Function Parameters Previous Hint"
	nextHintCommand      = "Function Parameters Next Hint"
)

func isCalltipCommand(command string) bool {
	return command == parameterHintCommand || command == previousHintCommand || command == nextHintCommand
}

type calltipState struct {
	doc      *document.Document
	pane     int
	language string
	catalog  *completion.FunctionCatalog
	source   string
	caret    int
	call     completion.CallHint
	overload int
}

func (state *model) startCalltip() {
	state.calltip = nil
	doc := state.paneDocument(state.activePane)
	if doc.Editor.CompositionActive() || doc.Editor.Selection().Start != doc.Editor.Selection().End {
		return
	}
	language := state.languageFor(doc)
	catalog, err := state.functionCatalog(language)
	if err != nil {
		state.notice = "Function parameters: " + err.Error()
		return
	}
	if catalog == nil {
		state.notice = "No API function catalog for this language"
		return
	}
	state.calltip = &calltipState{doc: doc, pane: state.activePane, language: language, catalog: catalog, caret: -1}
	if state.refreshCalltip() == nil {
		state.notice = "No function parameters at the caret"
	}
}

func (state *model) refreshCalltip() *calltipState {
	hint := state.calltip
	if hint == nil {
		return nil
	}
	editor := hint.doc.Editor
	selection := editor.Selection()
	if hint.pane != state.activePane || state.paneDocument(hint.pane) != hint.doc ||
		state.languageFor(hint.doc) != hint.language || editor.CompositionActive() || selection.Start != selection.End {
		state.calltip = nil
		return nil
	}
	value, caret := editor.String(), editor.ByteOffset(editor.Caret())
	if hint.caret != caret || hint.source != value {
		language, _ := syntax.LanguageByMode(hint.language)
		call := hint.catalog.CallAt(value, caret, completion.CallSyntax{
			LineComment:         language.LineComment,
			TripleQuotes:        language.NPPName == "python",
			NestedBlockComments: language.NPPName == "rust" || language.NPPName == "swift",
		})
		if len(call.Overloads) == 0 || hint.call.Name != "" && (hint.call.Open != call.Open || hint.call.Name != call.Name) {
			state.calltip = nil
			return nil
		}
		hint.source, hint.caret, hint.call = value, caret, call
	}
	return hint
}

func (state *model) cycleCalltip(step int) {
	if hint := state.refreshCalltip(); hint != nil {
		count := len(hint.call.Overloads)
		hint.overload = (hint.overload + count + step) % count
	}
}

func (state *model) paintCalltip(canvas *gfx.Canvas, size gfx.Size) {
	hint := state.refreshCalltip()
	if hint == nil || state.wordCompletion != nil || state.editorCommandFocusBlocked() || state.menuOverflowOpen {
		return
	}
	caret, ok := state.codeEditorForPane(hint.pane).CaretBounds()
	if !ok {
		return
	}
	signature := hint.call.Overloads[hint.overload]
	label := hint.call.Name + "(" + strings.Join(signature.Params, ", ") + ")"
	if signature.Return != "" {
		label += " -> " + signature.Return
	}
	argument := ""
	if hint.call.Argument < len(signature.Params) {
		argument = fmt.Sprintf("Parameter %d: %s", hint.call.Argument+1, signature.Params[hint.call.Argument])
	}
	label = strings.Join(strings.Fields(label), " ")
	argument = strings.Join(strings.Fields(argument), " ")
	count := len(hint.call.Overloads)
	controls := float32(0)
	if count > 1 {
		controls = 82
	}
	width := min(max(160, max(canvas.TextWidth(label, 11)+controls, canvas.TextWidth(argument, 11))+20), size.Width-8)
	height := float32(28)
	if argument != "" {
		height += 20
	}
	x := min(max(4, caret.X), size.Width-width-4)
	y := caret.Y + caret.Height + 2
	if y+height > size.Height-statusHeight-2 {
		y = max(4, caret.Y-height-2)
	}
	canvas.PushLayer(3)
	box := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(box, 1, palette.border, palette.white)
	row := gfx.Rect{X: x + 8, Y: y, Width: width - 16 - controls, Height: 27}
	smallLabel(canvas, canvas.FitLine(label, 11, gfx.WeightRegular, row.Width), row, palette.text, 11)
	canvas.HitWithOptions(calltipLabelID, row, label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	if argument != "" {
		row = gfx.Rect{X: x + 8, Y: y + 24, Width: width - 16, Height: 22}
		smallLabel(canvas, canvas.FitLine(argument, 11, gfx.WeightSemibold, row.Width), row, palette.blue, 11)
		canvas.HitWithOptions(calltipArgumentID, row, argument, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	}
	if count > 1 {
		controlsX := x + width - controls - 4
		position := fmt.Sprintf("%d/%d", hint.overload+1, count)
		canvas.LabelCenter(canvas.FitLine(position, 10, gfx.WeightRegular, 42), gfx.Rect{X: controlsX + 18, Y: y + 2, Width: 46, Height: 24}, gfx.TextStyle{Size: 10, Color: palette.text})
		for _, control := range []struct {
			id    gfx.ID
			x     float32
			icon  gfx.Icon
			label string
			step  int
		}{
			{calltipPreviousID, controlsX, leftIcon, "Previous overload", -1},
			{calltipNextID, controlsX + 64, rightIcon, "Next overload", 1},
		} {
			bounds := gfx.Rect{X: control.x, Y: y + 4, Width: 18, Height: 19}
			if canvas.Hovered() == control.id {
				canvas.Fill(bounds, palette.hover)
			}
			canvas.DrawIcon(control.icon, bounds.Inset(gfx.All(3)), palette.text)
			canvas.HitWithOptions(control.id, bounds, control.label, gfx.HitOptions{Role: gfx.RoleButton, Hint: control.label, SkipFocus: true})
			if canvas.Clicked(control.id) {
				state.cycleCalltip(control.step)
				canvas.RequestFocus(state.commandEditorFocusID())
			}
		}
	}
	canvas.PopLayer()
}
