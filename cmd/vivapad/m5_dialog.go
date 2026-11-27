package main

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/preferences"
)

var (
	m5DialogID = tree.ID(page, "m5-dialog")
	m5CancelID = tree.ID(page, "m5-cancel")
	m5Fields   = tree.Sub(page, tree.Name("m5-fields"))
)

type m5Dialog struct {
	kind        string
	fields      []*text.Editor
	index, size int
	styleRow    int
	theme       preferences.Theme
	message     string
	focus       bool
}

func (state *model) openM5Dialog(kind string, index int, values ...string) *m5Dialog {
	dialog := &m5Dialog{kind: kind, index: index, focus: true}
	for _, value := range values {
		dialog.fields = append(dialog.fields, text.NewEditor(value))
	}
	state.m5 = dialog
	state.menuOpen, state.menuOverflowOpen = -1, false
	state.wordCompletion, state.calltip = nil, nil
	state.cancelLiveComposition()
	state.invalidator.Invalidate()
	return dialog
}

func (state *model) closeM5Dialog() {
	state.m5 = nil
	state.requestNativeChooserFocus()
}

func (state *model) m5TextInput(id gfx.ID) *app.TextInput {
	for index, editor := range state.m5.fields {
		if id == m5Fields.At(index) {
			return &app.TextInput{Editor: editor}
		}
	}
	return nil
}

func (state *model) paintM5Dialog(canvas *gfx.Canvas, size gfx.Size) {
	dialog := state.m5
	palette := state.palette()
	width, height := float32(530), float32(348)
	if dialog.kind == projectPanelCommand {
		width, height = 388, 158
	}
	width, height = min(width, size.Width-20), min(height, size.Height-20)
	panel := gfx.Rect{X: (size.Width - width) / 2, Y: (size.Height - height) / 2, Width: width, Height: height}
	viewport := gfx.Rect{Width: size.Width, Height: size.Height}
	canvas.SetEscapeTarget(m5CancelID)
	canvas.PushFocusScope(m5DialogID)
	canvas.TrapFocus(m5DialogID)
	canvas.PushLayer(5)
	canvas.Fill(viewport, gfx.RGBA(0, 0, 0, .3))
	canvas.HitWithOptions(dialogBackdropID, viewport, dialog.kind, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	canvas.PushLayer(6)
	canvas.RoundedBorder(panel, 4, 1, palette.border, palette.popup)
	inner := panel.Inset(gfx.All(16))
	switch dialog.kind {
	case styleConfiguratorCommand:
		state.paintStyleConfigurator(canvas, inner, dialog)
	case udlManagerCommand:
		state.paintUDLManager(canvas, inner, dialog)
	case projectPanelCommand:
		state.paintProjectName(canvas, inner, dialog)
	}
	if state.m5 == dialog && dialog.focus && len(dialog.fields) > 0 {
		canvas.RequestFocusOnce(m5Fields.At(0))
		dialog.focus = false
	}
	if state.m5 == dialog && palette.m5Button(canvas, m5CancelID, gfx.Rect{X: inner.X + inner.Width - 74, Y: inner.Y + inner.Height - 29, Width: 74, Height: 29}, "Cancel", false) {
		state.closeM5Dialog()
	}
	canvas.PopLayer()
	canvas.PopLayer()
	canvas.PopFocusScope()
}

func (state *model) paintM5Field(canvas *gfx.Canvas, bounds gfx.Rect, index int, label, placeholder string, invalid bool) {
	theme := state.uiTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM = bounds.Height, 12
	if invalid {
		theme.Background = state.palette().invalidInput
	}
	id := m5Fields.At(index)
	components.PaintInput(canvas, theme, bounds, components.InputSnapshot{ID: id, Label: label,
		Placeholder: placeholder, Value: state.m5.fields[index].String(), PlatformEditing: canvas.Focused() == id})
}

func (palette *themePalette) m5Button(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, label string, disabled bool) bool {
	background, ink := palette.toolbar, palette.text
	if disabled {
		ink = palette.muted
	} else if canvas.Hovered() == id || canvas.FocusVisible() == id {
		background = palette.hover
	}
	canvas.Border(bounds, 1, palette.border, background)
	canvas.LabelCenter(canvas.FitLine(label, 11, gfx.WeightRegular, bounds.Width-6), bounds, gfx.TextStyle{Size: 11, Color: ink})
	canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: disabled})
	return !disabled && canvas.Clicked(id)
}
