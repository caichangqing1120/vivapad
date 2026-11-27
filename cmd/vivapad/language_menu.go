package main

import (
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/syntax"
)

var languageCommands = syntax.Languages()
var udlManagerID = tree.ID(page, "udl-manager-command")

func (state *model) availableLanguageCommands() []syntax.Language {
	if len(state.udls) == 0 {
		return languageCommands
	}
	commands := make([]syntax.Language, 0, len(languageCommands)+len(state.udls))
	for _, definition := range state.udls {
		commands = append(commands, syntax.Language{Label: udlLabelPrefix + definition.Name, Mode: definition.Mode()})
	}
	return append(commands, languageCommands...)
}

func (state *model) restoreSessionLanguages(flags map[string]string) {
	for _, doc := range state.space.Documents {
		if mode, selected := flags[doc.Path]; selected {
			if !syntax.ValidUDLMode(mode) || state.udlForMode(mode) != nil {
				state.languages[doc] = mode
			}
		}
		state.configureCode(doc)
	}
}

func (state *model) paintLanguageMenu(canvas *gfx.Canvas, size gfx.Size, origin float32) {
	palette := state.palette()
	commands := state.availableLanguageCommands()
	width := min(float32(252), size.Width-8)
	x := min(origin, size.Width-width-4)
	available := size.Height - menuHeight - statusHeight - 30
	rowHeight := float32(22)
	scrolling := float32(len(commands))*rowHeight+8 > available
	visible := len(commands)
	controls := float32(0)
	if scrolling {
		controls = 44
		visible = max(1, int((available-controls-8)/rowHeight))
	}
	state.languageVisible = visible
	state.languageOffset = min(max(0, state.languageOffset), len(commands)-visible)
	height := float32(visible+1)*rowHeight + controls + 8
	y := menuHeight
	canvas.PushLayer(2)
	canvas.PushOverlayClip(gfx.Rect{Width: size.Width, Height: size.Height})
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.popup)
	canvas.HitWithOptions(languagePanelID, panel, "Languages", gfx.HitOptions{Role: gfx.RoleMenu})
	manager := gfx.Rect{X: x + 4, Y: y + 4, Width: width - 8, Height: rowHeight}
	if canvas.Hovered() == udlManagerID {
		canvas.Fill(manager, palette.hover)
	}
	smallLabel(canvas, udlManagerCommand, manager.Inset(gfx.Insets{Left: 9}), palette.text, 11)
	canvas.HitWithOptions(udlManagerID, manager, udlManagerCommand, gfx.HitOptions{Role: gfx.RoleMenuItem})
	if canvas.Clicked(udlManagerID) {
		state.command(udlManagerCommand)
	}
	if scrolling {
		for _, control := range []struct {
			id    gfx.ID
			icon  gfx.Icon
			label string
			y     float32
			step  int
		}{
			{languagePrevID, upIcon, "Previous languages", y + 4 + rowHeight, -1},
			{languageNextID, downIcon, "Next languages", y + height - 26, 1},
		} {
			bounds := gfx.Rect{X: x + 4, Y: control.y, Width: width - 8, Height: 22}
			if canvas.Hovered() == control.id {
				canvas.Fill(bounds, palette.hover)
			}
			canvas.DrawIcon(control.icon, gfx.Rect{X: bounds.X + bounds.Width/2 - 8, Y: bounds.Y + 3, Width: 16, Height: 16}, palette.muted)
			canvas.HitWithOptions(control.id, bounds, control.label, gfx.HitOptions{Role: gfx.RoleMenuItem})
			if canvas.Clicked(control.id) {
				state.languageOffset = min(max(0, state.languageOffset+control.step), len(commands)-visible)
			}
		}
	}
	selected := -1
	for row := range visible {
		index := state.languageOffset + row
		spec := commands[index]
		item := gfx.Rect{X: x + 4, Y: y + 4 + controls/2 + float32(row+1)*rowHeight, Width: width - 8, Height: rowHeight}
		id := menuItemScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		checked := state.menuSelected(spec.Label)
		if checked {
			canvas.DrawIcon(checkIcon, gfx.Rect{X: item.X + 5, Y: item.Y + 4, Width: 14, Height: 14}, palette.accent)
		}
		label := canvas.FitLine(spec.Label, 11, gfx.WeightRegular, item.Width-36)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 26, Y: item.Y, Width: item.Width - 32, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, spec.Label, gfx.HitOptions{Role: gfx.RoleMenuItem, Checked: checked})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopOverlayClip()
	canvas.PopLayer()
	if selected >= 0 {
		state.command(commands[selected].Label)
	}
}
