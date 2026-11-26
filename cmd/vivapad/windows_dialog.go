package main

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

var windowsColumns = [...]string{"Name", "Path", "Type", "Length", "Modified"}

func (state *model) openWindowsDialog() {
	if state.pendingClose >= 0 || state.pendingReload >= 0 || state.filesDialogOpen || state.replaceConfirmOpen || state.renameDoc != nil || state.renamePlan != nil {
		state.notice = "Finish the open dialog first"
		return
	}
	state.windowsDialogOpen = true
	state.windowsDialogPane = state.activePane
	state.windowsRows = append(state.windowsRows[:0], state.activeViewTabs()...)
	state.windowsSelected = make(map[*document.Document]bool)
	state.windowsFocused = state.paneDocument(state.activePane)
	state.windowsOffset, state.windowsMessage, state.windowsHasSort = 0, "", false
}

func (state *model) closeWindowsDialog() {
	state.windowsDialogOpen = false
	state.windowsRows = nil
	state.windowsSelected = nil
	state.windowsFocused = nil
	state.windowsMessage = ""
}

func (state *model) refreshWindowsRows() {
	tabs := state.activeViewTabs()
	if !state.windowsHasSort {
		state.windowsRows = append(state.windowsRows[:0], tabs...)
		return
	}
	if err := state.sortWindowsRows(tabs, state.windowsSort); err != nil {
		state.windowsMessage = "Cannot sort: " + err.Error()
		// Keep the last order while removing closed tabs and appending new ones.
		open := make(map[*document.Document]bool, len(tabs))
		for _, doc := range tabs {
			open[doc] = true
		}
		state.windowsRows = slices.DeleteFunc(state.windowsRows, func(doc *document.Document) bool {
			if !open[doc] {
				return true
			}
			delete(open, doc)
			return false
		})
		for _, doc := range tabs {
			if open[doc] {
				state.windowsRows = append(state.windowsRows, doc)
			}
		}
	}
}

func (state *model) sortWindowsRows(tabs []*document.Document, command windowSortCommand) error {
	keys := make(map[*document.Document]windowSortKey, len(tabs))
	for _, doc := range tabs {
		key, err := state.windowKey(doc, command.column == 4)
		if err != nil {
			return err
		}
		keys[doc] = key
	}
	state.windowsRows = append(state.windowsRows[:0], tabs...)
	slices.SortStableFunc(state.windowsRows, func(first, second *document.Document) int {
		if state.isPinned(first) != state.isPinned(second) {
			if state.isPinned(first) {
				return -1
			}
			return 1
		}
		result := compareWindowKeys(keys[first], keys[second], command.column)
		if command.reverse {
			return -result
		}
		return result
	})
	return nil
}

func (state *model) sortWindowsDialogBy(column int) {
	command := windowSortCommand{column: column, reverse: state.windowsHasSort && state.windowsSort.column == column && !state.windowsSort.reverse}
	if err := state.sortWindowsRows(state.activeViewTabs(), command); err != nil {
		state.windowsMessage = "Cannot sort: " + err.Error()
		return
	}
	state.windowsSort, state.windowsHasSort = command, true
	state.windowsOffset, state.windowsMessage = 0, ""
}

func (state *model) activateWindowsDialogDocument() bool {
	doc := state.windowsFocused
	index := state.documentIndex(doc)
	if doc == nil || index < 0 {
		state.windowsMessage = "Select a window to activate"
		return false
	}
	state.closeWindowsDialog()
	state.activePane = state.windowsDialogPane
	state.paneDocs[state.activePane] = doc
	state.space.Select(index)
	return true
}

func (state *model) saveWindowsDialogSelection() {
	selected := 0
	for _, doc := range state.windowsRows {
		if !state.windowsSelected[doc] {
			continue
		}
		selected++
		if index := state.documentIndex(doc); index >= 0 && !state.save(index, false) {
			state.windowsMessage = "Save canceled or failed"
			return
		}
	}
	if selected == 0 {
		state.windowsMessage = "Select windows to save"
	} else {
		state.windowsMessage = "Saved " + strconv.Itoa(selected) + " window(s)"
	}
}

func (state *model) closeWindowsDialogSelection() {
	var targets []closeTarget
	docs := state.activeViewTabs()
	for index := len(docs) - 1; index >= 0; index-- {
		if state.windowsSelected[docs[index]] {
			targets = append(targets, closeTarget{doc: docs[index], pane: state.windowsDialogPane})
		}
	}
	if len(targets) == 0 {
		state.windowsMessage = "Select windows to close"
		return
	}
	keep := state.paneDocument(state.windowsDialogPane)
	if state.windowsSelected[keep] {
		keep = nil
	}
	state.closeWindowsDialog()
	state.startBatchTargets(targets, keep)
}

func (state *model) applyWindowsDialogSort() {
	command := state.windowsSort
	if !state.windowsHasSort {
		command = windowSortCommand{column: 0}
	}
	if err := state.sortWindowTabs(command); err != nil {
		state.windowsMessage = "Sort failed: " + err.Error()
		return
	}
	state.windowsSort, state.windowsHasSort = command, true
	state.windowsMessage = "Tabs sorted"
	state.refreshWindowsRows()
}

func (state *model) scrollWindowsDialog(event gfx.ScrollEvent) gfx.EventResult {
	step := 1
	if event.StepY(23) < 0 {
		step = -1
	}
	state.windowsOffset = min(max(0, state.windowsOffset+step), max(0, len(state.windowsRows)-state.windowsVisible))
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (state *model) windowsDialogKey(event gfx.KeyEvent) gfx.EventResult {
	switch event.Key {
	case gfx.KeyEscape:
		state.closeWindowsDialog()
	case gfx.KeyEnter:
		if state.activateWindowsDialogDocument() {
			return gfx.EventResult{Handled: true, Repaint: true, Focus: state.commandEditorFocusID()}
		}
	case gfx.KeyDown, gfx.KeyUp:
		index := slices.Index(state.windowsRows, state.windowsFocused)
		if event.Key == gfx.KeyDown {
			index++
		} else {
			index--
		}
		if len(state.windowsRows) != 0 {
			index = min(max(0, index), len(state.windowsRows)-1)
			state.windowsFocused = state.windowsRows[index]
			if index < state.windowsOffset {
				state.windowsOffset = index
			} else if index >= state.windowsOffset+state.windowsVisible {
				state.windowsOffset = index - state.windowsVisible + 1
			}
		}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

func windowDialogColumns(width float32) [5]float32 {
	return [5]float32{width * 0.25, width * 0.34, width * 0.17, width * 0.11, width * 0.13}
}

func (state *model) paintWindowsDialog(canvas *gfx.Canvas, size gfx.Size) {
	state.refreshWindowsRows()
	canvas.SetEscapeTarget(windowsOKID)
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Windows dialog", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width, height := min(float32(620), size.Width-24), min(float32(360), max(260, float32(len(state.windowsRows))*23+150), size.Height-28)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	title := "Windows - " + strconv.Itoa(len(state.windowsRows)) + " documents"
	smallLabel(canvas, title, gfx.Rect{X: x + 14, Y: y + 10, Width: width - 28, Height: 27}, palette.text, 15)
	list := gfx.Rect{X: x + 12, Y: y + 45, Width: width - 143, Height: height - 83}
	canvas.Border(list, 1, palette.border, palette.white)
	columns := windowDialogColumns(list.Width)
	columnX := list.X
	for index, label := range windowsColumns {
		bounds := gfx.Rect{X: columnX, Y: list.Y + 1, Width: columns[index], Height: 24}
		id := windowsHeaderScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(bounds, palette.hover)
		}
		short := canvas.FitLine(label, 10, gfx.WeightRegular, bounds.Width-8)
		smallLabel(canvas, short, gfx.Rect{X: bounds.X + 4, Y: bounds.Y, Width: bounds.Width - 8, Height: bounds.Height}, palette.text, 10)
		canvas.HitWithOptions(id, bounds, "Sort by "+label, gfx.HitOptions{Role: gfx.RoleButton})
		if canvas.Clicked(id) {
			state.sortWindowsDialogBy(index)
		}
		columnX += columns[index]
	}
	canvas.Fill(gfx.Rect{X: list.X, Y: list.Y + 25, Width: list.Width, Height: 1}, palette.border)
	rowArea := gfx.Rect{X: list.X + 1, Y: list.Y + 26, Width: list.Width - 2, Height: list.Height - 54}
	state.windowsVisible = max(1, int(rowArea.Height/23))
	state.windowsOffset = min(max(0, state.windowsOffset), max(0, len(state.windowsRows)-state.windowsVisible))
	canvas.HitWithOptions(windowsPanelID, rowArea, "Open windows", gfx.HitOptions{Role: gfx.RoleGroup})
	canvas.Clip(rowArea)
	for index := state.windowsOffset; index < len(state.windowsRows) && index-state.windowsOffset < state.windowsVisible; index++ {
		doc := state.windowsRows[index]
		item := gfx.Rect{X: rowArea.X, Y: rowArea.Y + float32(index-state.windowsOffset)*23, Width: rowArea.Width, Height: 23}
		id := windowsRowScope.At(index)
		selected := state.windowsSelected[doc]
		if selected || canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		} else if doc == state.windowsFocused {
			canvas.Fill(item, gfx.RGB(242, 243, 245))
		}
		box := gfx.Rect{X: item.X + 4, Y: item.Y + 5, Width: 13, Height: 13}
		canvas.Border(box, 1, palette.border, palette.white)
		if selected {
			canvas.DrawIcon(checkIcon, box.Inset(gfx.All(1)), palette.blue)
		}
		directory, _ := filepath.Split(doc.Path)
		modified := "-"
		if doc.Path != "" {
			if info, err := os.Stat(doc.Path); err == nil {
				modified = info.ModTime().Format("01/02 15:04")
			}
		}
		cells := [5]string{doc.Name, directory, state.languageLabel(doc), strconv.Itoa(windowContentLength(doc)), modified}
		cellX := item.X
		for column, content := range cells {
			indent := float32(3)
			if column == 0 {
				indent = 22
			}
			fit := canvas.FitLine(content, 10, gfx.WeightRegular, columns[column]-indent-4)
			smallLabel(canvas, fit, gfx.Rect{X: cellX + indent, Y: item.Y, Width: columns[column] - indent - 4, Height: item.Height}, palette.text, 10)
			cellX += columns[column]
		}
		canvas.HitWithOptions(id, item, "Select "+doc.Name, gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: selected, Hint: doc.Path})
		if canvas.Clicked(id) {
			if selected {
				delete(state.windowsSelected, doc)
			} else {
				state.windowsSelected[doc] = true
			}
			state.windowsFocused = doc
		}
	}
	canvas.Unclip()
	for _, button := range []struct {
		id    gfx.ID
		icon  gfx.Icon
		label string
		x     float32
		step  int
	}{
		{windowsPreviousID, upIcon, "Previous windows", list.X + 3, -1},
		{windowsNextID, downIcon, "Next windows", list.X + list.Width - 27, 1},
	} {
		bounds := gfx.Rect{X: button.x, Y: list.Y + list.Height - 25, Width: 24, Height: 22}
		canvas.DrawIcon(button.icon, bounds.Inset(gfx.All(4)), palette.muted)
		if hitButton(canvas, button.id, bounds, button.label) {
			state.windowsOffset = min(max(0, state.windowsOffset+button.step), max(0, len(state.windowsRows)-state.windowsVisible))
		}
	}
	if state.windowsMessage != "" {
		message := canvas.FitLine(state.windowsMessage, 10, gfx.WeightRegular, list.Width-62)
		smallLabel(canvas, message, gfx.Rect{X: list.X + 31, Y: list.Y + list.Height - 25, Width: list.Width - 62, Height: 22}, palette.muted, 10)
	}
	for index, button := range []struct {
		id    gfx.ID
		label string
	}{
		{windowsActivateID, "Activate"},
		{windowsSaveID, "Save"},
		{windowsCloseID, "Close window(s)"},
		{windowsSortTabsID, "Sort tabs"},
		{windowsOKID, "OK"},
	} {
		bounds := gfx.Rect{X: x + width - 117, Y: y + 47 + float32(index)*40, Width: 102, Height: 29}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		label := canvas.FitLine(button.label, 11, gfx.WeightRegular, bounds.Width-12)
		canvas.LabelCenter(label, bounds, gfx.TextStyle{Size: 11, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			switch button.id {
			case windowsActivateID:
				if state.activateWindowsDialogDocument() {
					canvas.RequestFocus(state.commandEditorFocusID())
				}
			case windowsSaveID:
				state.saveWindowsDialogSelection()
			case windowsCloseID:
				state.closeWindowsDialogSelection()
			case windowsSortTabsID:
				state.applyWindowsDialogSort()
			case windowsOKID:
				state.closeWindowsDialog()
			}
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}
