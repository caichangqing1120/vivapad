package main

import "github.com/vivaui/viva/gfx"

var productivityScope = tree.Sub(page, tree.Name("productivity-menu"))

func productivityCommands(parent string) []string {
	var tools []textTool
	switch parent {
	case structuredToolsMenu:
		tools = structuredTools
	case encodingToolsMenu:
		tools = encodingTools
	case compareMenu:
		return compareCommands
	default:
		return nil
	}
	commands := make([]string, len(tools))
	for index, tool := range tools {
		commands[index] = tool.command
	}
	return commands
}

func (state *model) openProductivityMenu(command string, from int) bool {
	switch command {
	case structuredToolsMenu, encodingToolsMenu:
		state.menuOpen = 7
	case compareMenu:
		state.menuOpen = 7
		if from == 3 {
			state.menuOpen = 3
		}
	default:
		return false
	}
	state.productivityMenu = command
	return true
}

func (state *model) productivityMenuKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if state.menuOpen != 3 && state.menuOpen != 7 || event.Modifiers != 0 {
		return gfx.EventResult{}, false
	}
	commands := menuCommands[state.menuOpen]
	parent := state.productivityMenu
	var parentID gfx.ID
	for index, label := range commands {
		id := menuItemScope.At(index)
		if event.Target == id && (label == compareMenu || label == structuredToolsMenu || label == encodingToolsMenu) {
			if event.Key == gfx.KeyRight || event.Key == gfx.KeyEnter {
				state.command(label)
				return gfx.EventResult{Handled: true, Repaint: true, Focus: productivityScope.At(0), FocusVisible: true}, true
			}
		}
		if label == parent {
			parentID = id
		}
	}
	items := productivityCommands(parent)
	if len(items) == 0 {
		return gfx.EventResult{}, false
	}
	if event.Key == gfx.KeyEscape || event.Key == gfx.KeyLeft && event.Target.Scope() == productivityScope {
		state.productivityMenu = ""
		return gfx.EventResult{Handled: true, Repaint: true, Focus: parentID, FocusVisible: true}, true
	}
	if event.Target.Scope() != productivityScope {
		return gfx.EventResult{}, false
	}
	index := -1
	for candidate := range items {
		if event.Target == productivityScope.At(candidate) {
			index = candidate
			break
		}
	}
	switch event.Key {
	case gfx.KeyEnter:
		if index >= 0 {
			state.command(items[index])
			return gfx.EventResult{Handled: true, Repaint: true, Focus: state.commandEditorFocusID()}, true
		}
	case gfx.KeyUp:
		index = (max(0, index) + len(items) - 1) % len(items)
	case gfx.KeyDown:
		index = (index + 1) % len(items)
	case gfx.KeyHome:
		index = 0
	case gfx.KeyEnd:
		index = len(items) - 1
	default:
		return gfx.EventResult{}, false
	}
	return gfx.EventResult{Handled: true, Repaint: true, Focus: productivityScope.At(max(0, index)), FocusVisible: true}, true
}

func (state *model) paintProductivityMenu(canvas *gfx.Canvas, size gfx.Size, parentLeft, parentRight, parentY float32) {
	items := productivityCommands(state.productivityMenu)
	if len(items) == 0 {
		return
	}
	palette := state.palette()
	width := min(float32(232), size.Width-8)
	rowHeight := min(float32(27), max(1, (size.Height-menuHeight-statusHeight-12)/float32(len(items))))
	height := float32(len(items))*rowHeight + 8
	x := parentRight
	if x+width > size.Width-2 {
		x = max(2, parentLeft-width)
	}
	y := min(parentY, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.popup)
	hot := canvas.Hovered()
	if focus := canvas.FocusVisible(); focus.Scope() == productivityScope {
		hot = focus
	}
	selected := ""
	for index, label := range items {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*rowHeight, Width: width - 8, Height: rowHeight}
		id := productivityScope.At(index)
		if hot == id {
			canvas.Fill(item, palette.hover)
		}
		labelWidth := max(0, item.Width-18)
		smallLabel(canvas, canvas.FitLine(label, 11, gfx.WeightRegular, labelWidth), gfx.Rect{X: item.X + 9, Y: item.Y, Width: labelWidth, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, label, gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = label
		}
	}
	canvas.PopLayer()
	if selected != "" {
		state.command(selected)
	}
}
