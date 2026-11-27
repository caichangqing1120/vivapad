package main

import (
	"cmp"
	"runtime"
	"slices"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/shortcuts"
)

const shortcutMapperCommand = "Shortcut Mapper..."

var (
	mapperSearchID  = tree.ID(page, "shortcut-search")
	mapperCaptureID = tree.ID(page, "shortcut-capture")
	mapperClearID   = tree.ID(page, "shortcut-clear")
	mapperResetID   = tree.ID(page, "shortcut-reset")
	mapperResetAll  = tree.ID(page, "shortcut-reset-all")
	mapperApplyID   = tree.ID(page, "shortcut-apply")
	mapperCloseID   = tree.ID(page, "shortcut-close")
	mapperPrevID    = tree.ID(page, "shortcut-previous")
	mapperNextID    = tree.ID(page, "shortcut-next")
	mapperPanelID   = tree.ID(page, "shortcut-list")
	mapperDialogID  = tree.ID(page, "shortcut-dialog")
	mapperRows      = tree.Sub(page, tree.Name("shortcut-rows"))
)

type shortcutMapper struct {
	search              *text.Editor
	catalog, rows       []shortcuts.Command
	filter, selected    string
	message             string
	draft               shortcuts.Chord
	pending, capturing  bool
	failed, focusSearch bool
	offset, visible     int
}

func (state *model) shortcutCatalog() []shortcuts.Command {
	catalog := shortcuts.Defaults()
	seen := make(map[string]bool)
	for _, command := range catalog {
		seen[command.ID] = true
	}
	add := func(id, name string) {
		if id != "" && !seen[id] {
			catalog = append(catalog, shortcuts.Command{ID: id, Name: name})
			seen[id] = true
		}
	}
	for _, menu := range menuCommands {
		for _, command := range menu {
			add(command, command)
		}
	}
	for _, commands := range [][]string{
		productivityCommands(structuredToolsMenu), productivityCommands(encodingToolsMenu), compareCommands,
		lineCommands[:], caseCommands[:], commentCommands[:], viewTabCommands[:], eolCommands[:],
		bookmarkCommands[:], blankCommands[:], copyPathsCommands[:], symbolCommands[:],
		insertDateCommands[:], sessionCommands[:], closeMultipleCommands[:], readOnlyCommands[:],
		autoCompletionCommands[:], selectedSearchCommands[:], tabColorCommands[:],
	} {
		for _, command := range commands {
			add(command, command)
		}
	}
	for _, language := range languageCommands {
		add(language.Label, language.Label)
	}
	for _, item := range state.namedMacros {
		add("macro:"+item.Name, "Macro: "+item.Name)
	}
	for _, item := range state.namedRuns {
		add("run:"+item.Name, "Run: "+item.Name)
	}
	for index := range catalog {
		if catalog[index].Name == "" {
			catalog[index].Name = catalog[index].ID
		}
	}
	slices.SortFunc(catalog, func(left, right shortcuts.Command) int {
		return cmp.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	return catalog
}

func (state *model) openShortcutMapper() {
	catalog := state.shortcutCatalog()
	err := state.shortcuts.Register(catalog)
	state.mapper = &shortcutMapper{search: text.NewEditor(""), catalog: catalog, focusSearch: true}
	if err != nil {
		state.mapper.message, state.mapper.failed = err.Error(), true
	}
	state.menuOverflowOpen = false
	state.wordCompletion, state.calltip = nil, nil
}

func formatShortcut(binding shortcuts.Chord) string {
	if binding.Empty() {
		return "Unassigned"
	}
	return binding.Format(runtime.GOOS)
}

func (state *model) refreshMapperRows() {
	mapper := state.mapper
	query := strings.ToLower(strings.TrimSpace(mapper.search.String()))
	if query != mapper.filter {
		mapper.filter, mapper.offset = query, 0
	}
	mapper.rows = mapper.rows[:0]
	for _, command := range mapper.catalog {
		binding := state.shortcuts.Binding(command.ID)
		match := strings.Contains(strings.ToLower(command.Name), query) ||
			strings.Contains(strings.ToLower(formatShortcut(binding)), query) ||
			strings.Contains(strings.ToLower(binding.Format("windows")), query)
		if !match && binding == command.Default {
			for _, alias := range command.Aliases {
				match = match || strings.Contains(strings.ToLower(alias.Format(runtime.GOOS)), query)
			}
		}
		if match {
			mapper.rows = append(mapper.rows, command)
		}
	}
}

func (state *model) closeShortcutMapper() {
	state.mapper = nil
	state.requestNativeChooserFocus()
}

func (state *model) stageShortcut(binding shortcuts.Chord) {
	mapper := state.mapper
	mapper.draft, mapper.pending, mapper.capturing = binding, true, false
	mapper.message, mapper.failed = "", false
	if err := state.shortcuts.Conflict(mapper.selected, binding); err != nil {
		mapper.message, mapper.failed = err.Error(), true
	}
}

func (state *model) updateShortcut(action gfx.ID) {
	mapper := state.mapper
	var err error
	switch action {
	case mapperApplyID:
		if !mapper.pending || mapper.failed {
			return
		}
		err = state.shortcuts.Set(mapper.selected, mapper.draft)
	case mapperResetID:
		err = state.shortcuts.Reset(mapper.selected)
	case mapperResetAll:
		err = state.shortcuts.ResetAll()
	}
	mapper.capturing = false
	if err != nil {
		mapper.message, mapper.failed = err.Error(), true
		return
	}
	mapper.message, mapper.failed, mapper.pending = "Shortcuts saved", false, false
}

func (state *model) shortcutMapperKey(event gfx.KeyEvent, printable bool) gfx.EventResult {
	mapper := state.mapper
	result := gfx.EventResult{Handled: true, Repaint: true}
	if event.Release {
		return result
	}
	if event.Key == gfx.KeyEscape {
		state.closeShortcutMapper()
		result.Focus = state.commandEditorFocusID()
		return result
	}
	if mapper.capturing {
		if event.Key.IsModifier() || event.Repeat {
			return result
		}
		if binding, ok := eventShortcut(event); ok {
			state.stageShortcut(binding)
		} else {
			mapper.message, mapper.failed = "Use a function key or a modified key", true
		}
		return result
	}
	if event.Target == mapperSearchID || !printable {
		return gfx.EventResult{}
	}
	return result
}

func (state *model) paintShortcutMapper(canvas *gfx.Canvas, size gfx.Size) {
	mapper := state.mapper
	state.refreshMapperRows()
	palette := state.palette()
	viewport := gfx.Rect{Width: size.Width, Height: size.Height}
	width, height := min(670, size.Width-32), min(568, size.Height-32)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.SetEscapeTarget(mapperCloseID)
	canvas.PushFocusScope(mapperDialogID)
	canvas.TrapFocus(mapperDialogID)
	canvas.PushLayer(5)
	canvas.Fill(viewport, gfx.RGBA(0, 0, 0, .3))
	canvas.HitWithOptions(dialogBackdropID, viewport, "Shortcut Mapper", gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	canvas.PushLayer(6)
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.popup)
	left, inner := x+16, width-32
	smallLabel(canvas, "Shortcut Mapper", gfx.Rect{X: left, Y: y + 10, Width: inner, Height: 24}, palette.text, 15)
	theme := state.uiTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 30, 12, 2
	components.PaintInput(canvas, theme, gfx.Rect{X: left, Y: y + 40, Width: inner, Height: 30}, components.InputSnapshot{
		ID: mapperSearchID, Label: "Search shortcuts", Placeholder: "Search commands or keys", Value: mapper.search.String(), PlatformEditing: canvas.Focused() == mapperSearchID,
	})
	if mapper.focusSearch {
		canvas.RequestFocusOnce(mapperSearchID)
		mapper.focusSearch = false
	}
	if canvas.Clicked(mapperSearchID) {
		mapper.capturing = false
	}
	list := gfx.Rect{X: left, Y: y + 78, Width: inner, Height: height - 182}
	canvas.Border(list, 1, palette.border, palette.popup)
	canvas.HitWithOptions(mapperPanelID, list, "Shortcut commands", gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	shortWidth := min(float32(190), inner/3)
	nameWidth := inner - shortWidth - 28
	canvas.Fill(gfx.Rect{X: left + 1, Y: list.Y + 1, Width: inner - 2, Height: 24}, palette.toolbar)
	smallLabel(canvas, "Command", gfx.Rect{X: left + 8, Y: list.Y + 1, Width: nameWidth, Height: 23}, palette.muted, 11)
	smallLabel(canvas, "Shortcut", gfx.Rect{X: left + 8 + nameWidth, Y: list.Y + 1, Width: shortWidth, Height: 23}, palette.muted, 11)
	const rowHeight = 23
	mapper.visible = max(1, int((list.Height-26)/rowHeight))
	maximum := max(0, len(mapper.rows)-mapper.visible)
	mapper.offset = min(max(0, mapper.offset), maximum)
	for row, command := range mapper.rows[mapper.offset:min(len(mapper.rows), mapper.offset+mapper.visible)] {
		rowID := mapperRows.At(mapper.offset + row)
		bounds := gfx.Rect{X: left + 1, Y: list.Y + 25 + float32(row)*rowHeight, Width: inner - 24, Height: rowHeight}
		binding := state.shortcuts.Binding(command.ID)
		selected := command.ID == mapper.selected
		if selected || canvas.FocusVisible() == rowID {
			canvas.Fill(bounds, palette.selectedRow)
		} else if canvas.Hovered() == rowID {
			canvas.Fill(bounds, palette.hover)
		}
		label := formatShortcut(binding)
		if command.Fixed {
			label += "  ·  System"
		}
		color := palette.text
		if selected && mapper.pending {
			label = formatShortcut(mapper.draft)
			if mapper.failed {
				color = palette.diff[0]
			}
		}
		smallLabel(canvas, canvas.FitLine(command.Name, 12, gfx.WeightRegular, nameWidth-8), gfx.Rect{X: bounds.X + 7, Y: bounds.Y, Width: nameWidth - 8, Height: rowHeight}, palette.text, 12)
		smallLabel(canvas, canvas.FitLine(label, 12, gfx.WeightRegular, shortWidth), gfx.Rect{X: bounds.X + 7 + nameWidth, Y: bounds.Y, Width: shortWidth, Height: rowHeight}, color, 12)
		canvas.HitWithOptions(rowID, bounds, command.Name, gfx.HitOptions{Role: gfx.RoleButton, Hint: label, Selected: selected})
		if canvas.Clicked(rowID) && !selected {
			mapper.selected, mapper.message = command.ID, ""
			mapper.pending, mapper.capturing, mapper.failed = false, false, false
		}
	}
	if len(mapper.rows) == 0 {
		smallLabel(canvas, "No matching commands", gfx.Rect{X: left + 8, Y: list.Y + 25, Width: inner - 16, Height: rowHeight}, palette.muted, 12)
	}
	for _, control := range []struct {
		id    gfx.ID
		icon  gfx.Icon
		label string
		y     float32
		step  int
	}{{mapperPrevID, upIcon, "Previous shortcuts", list.Y + 25, -1}, {mapperNextID, downIcon, "Next shortcuts", list.Y + list.Height - 23, 1}} {
		bounds := gfx.Rect{X: left + inner - 23, Y: control.y, Width: 22, Height: 22}
		canvas.DrawIcon(control.icon, controlIconBounds(bounds), palette.muted)
		if hitButton(canvas, control.id, bounds, control.label) {
			mapper.offset = min(max(0, mapper.offset+control.step), maximum)
		}
	}
	message := mapper.message
	if mapper.capturing && message == "" {
		message = "Press a shortcut"
	}
	locked := mapper.selected == "" || nativeEditShortcut(mapper.selected)
	messageColor := palette.muted
	if mapper.failed {
		messageColor = palette.diff[0]
	}
	smallLabel(canvas, canvas.FitLine(message, 12, gfx.WeightRegular, inner), gfx.Rect{X: left, Y: y + height - 95, Width: inner, Height: 23}, messageColor, 12)
	buttons := []struct {
		id       gfx.ID
		label    string
		disabled bool
	}{
		{mapperCaptureID, "Record Keys", locked}, {mapperClearID, "Clear", locked},
		{mapperResetID, "Reset Binding", locked}, {mapperResetAll, "Reset All", false},
		{mapperApplyID, "Apply", !mapper.pending || mapper.failed}, {mapperCloseID, "Close", false},
	}
	for index, button := range buttons {
		buttonWidth := (inner - 5*6) / 6
		bounds := gfx.Rect{X: left + float32(index)*(buttonWidth+6), Y: y + height - 51, Width: buttonWidth, Height: 30}
		color, background := palette.text, palette.toolbar
		if button.disabled {
			color = palette.muted
		} else if canvas.Hovered() == button.id || canvas.Focused() == button.id || button.id == mapperCaptureID && mapper.capturing {
			background = palette.hover
		}
		canvas.Border(bounds, 1, palette.border, background)
		canvas.LabelCenter(button.label, bounds, gfx.TextStyle{Size: 12, Color: color})
		canvas.HitWithOptions(button.id, bounds, button.label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: button.disabled})
		if button.disabled || !canvas.Clicked(button.id) {
			continue
		}
		switch button.id {
		case mapperCaptureID:
			mapper.capturing, mapper.failed, mapper.pending, mapper.message = true, false, false, ""
			canvas.RequestFocusOnce(mapperCaptureID)
		case mapperClearID:
			state.stageShortcut(shortcuts.Chord{})
		case mapperCloseID:
			state.closeShortcutMapper()
			canvas.RequestFocusOnce(state.commandEditorFocusID())
		default:
			state.updateShortcut(button.id)
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
	canvas.PopFocusScope()
}
