package main

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/projects"
)

const projectPanelCommand = "Projects"

var (
	leftDockProjectsID = tree.ID(page, "left-dock-projects")
	projectsPanelID    = tree.ID(page, "projects-panel")
	projectsCloseID    = tree.ID(page, "projects-close")
	projectNewID       = tree.ID(page, "project-new")
	projectRenameID    = tree.ID(page, "project-rename")
	projectDeleteID    = tree.ID(page, "project-delete")
	projectAddID       = tree.ID(page, "project-add-current")
	projectSaveID      = tree.ID(page, "project-save-name")
	projectPrevID      = tree.ID(page, "project-scroll-previous")
	projectNextID      = tree.ID(page, "project-scroll-next")
	projectRows        = tree.Sub(page, tree.Name("project-rows"))
	projectRemoveRows  = tree.Sub(page, tree.Name("project-file-remove"))
)

type projectPanel struct {
	groups   []projects.Group
	visible  bool
	selected int
	offset   int
}

func (state *model) updateProjects(groups []projects.Group) error {
	if err := projects.Save(state.projectsPath, groups); err != nil {
		state.notice = "Projects not saved: " + err.Error()
		return err
	}
	state.projects.groups = groups
	state.invalidator.Invalidate()
	return nil
}

func (state *model) paintProjectName(canvas *gfx.Canvas, bounds gfx.Rect, dialog *m5Dialog) {
	palette := state.palette()
	label := "New Project"
	if dialog.index >= 0 {
		label = "Rename Project"
	}
	smallLabel(canvas, label, gfx.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: 23}, palette.text, 15)
	state.paintM5Field(canvas, gfx.Rect{X: bounds.X, Y: bounds.Y + 28, Width: bounds.Width, Height: 29}, 0, "Project name", "Project name", false)
	smallLabel(canvas, canvas.FitLine(dialog.message, 11, gfx.WeightRegular, bounds.Width), gfx.Rect{X: bounds.X, Y: bounds.Y + 61, Width: bounds.Width, Height: 24}, palette.diff[0], 11)
	button := gfx.Rect{X: bounds.X + bounds.Width - 156, Y: bounds.Y + bounds.Height - 29, Width: 74, Height: 29}
	if palette.m5Button(canvas, projectSaveID, button, "Save", false) {
		groups, index := slices.Clone(state.projects.groups), dialog.index
		name := strings.TrimSpace(dialog.fields[0].String())
		if index < 0 {
			index = len(groups)
			groups = append(groups, projects.Group{Name: name})
		} else {
			groups[index].Name = name
		}
		if err := state.updateProjects(groups); err != nil {
			dialog.message = err.Error()
			return
		}
		state.projects.selected, state.projects.offset = index, index
		state.closeM5Dialog()
	}
}

func (state *model) paintProjectPanel(canvas *gfx.Canvas, bounds gfx.Rect) {
	panel := &state.projects
	palette := state.palette()
	canvas.Fill(bounds, palette.dock)
	canvas.Fill(gfx.Rect{X: bounds.Width - 1, Y: bounds.Y, Width: 1, Height: bounds.Height}, palette.border)
	state.interfaceLabel(canvas, state.preferenceText("Projects"), gfx.Rect{X: 9, Y: bounds.Y + 3, Width: bounds.Width - 40, Height: 26}, palette.text, 12)
	closeBounds := gfx.Rect{X: bounds.Width - 25, Y: bounds.Y + 7, Width: 18, Height: 18}
	canvas.DrawIcon(closeIcon, closeBounds.Inset(gfx.All(3)), palette.muted)
	if hitButton(canvas, projectsCloseID, closeBounds, "Close Projects") {
		panel.visible = false
		state.activeLeftDock = state.activeDock()
		return
	}
	valid := panel.selected >= 0 && panel.selected < len(panel.groups)
	buttonWidth := (bounds.Width - 22) / 3
	button := gfx.Rect{X: 6, Y: bounds.Y + 33, Width: buttonWidth, Height: 24}
	if palette.m5Button(canvas, projectNewID, button, "New", false) {
		state.openM5Dialog(projectPanelCommand, -1, "")
	}
	button.X += buttonWidth + 5
	if palette.m5Button(canvas, projectRenameID, button, "Rename", !valid) {
		state.openM5Dialog(projectPanelCommand, panel.selected, panel.groups[panel.selected].Name)
	}
	button.X += buttonWidth + 5
	if palette.m5Button(canvas, projectDeleteID, button, "Delete", !valid) {
		groups := slices.Delete(slices.Clone(panel.groups), panel.selected, panel.selected+1)
		if state.updateProjects(groups) == nil {
			panel.selected = min(panel.selected, len(groups)-1)
			state.notice = "Project removed; files unchanged"
		}
		return
	}
	path := state.space.Current().Path
	add := gfx.Rect{X: 6, Y: bounds.Y + 62, Width: bounds.Width - 12, Height: 24}
	if palette.m5Button(canvas, projectAddID, add, "Add Current File", !valid || path == "") {
		group := panel.groups[panel.selected]
		if state.isPrivateDataPath(path) {
			state.notice = "Private configuration cannot be added to a project"
		} else if slices.ContainsFunc(group.Files, func(existing string) bool { return projects.FileKey(path) == projects.FileKey(existing) }) {
			state.notice = "File is already in this project"
		} else {
			groups := slices.Clone(panel.groups)
			groups[panel.selected].Files = append(slices.Clone(group.Files), path)
			if state.updateProjects(groups) == nil {
				state.notice = "File reference added"
			}
		}
	}
	const rowHeight = 24
	list := gfx.Rect{X: 3, Y: bounds.Y + 92, Width: bounds.Width - 6, Height: max(0, bounds.Height-92)}
	rows := len(panel.groups)
	if valid {
		rows += len(panel.groups[panel.selected].Files)
	}
	if rows == 0 {
		state.interfaceLabel(canvas, state.preferenceText("No projects"), gfx.Rect{X: 9, Y: list.Y, Width: list.Width - 6, Height: rowHeight}, palette.muted, 11)
		return
	}
	visible := max(1, int(list.Height/rowHeight))
	if rows > visible {
		visible = max(1, visible-1)
		prev := gfx.Rect{X: list.X, Y: list.Y + float32(visible)*rowHeight, Width: list.Width / 2, Height: rowHeight}
		next := prev
		next.X += next.Width
		if palette.m5Button(canvas, projectPrevID, prev, "Previous", panel.offset <= 0) {
			panel.offset--
		}
		if palette.m5Button(canvas, projectNextID, next, "Next", panel.offset >= rows-visible) {
			panel.offset++
		}
	}
	panel.offset = min(max(0, panel.offset), max(0, rows-visible))
	list.Height = min(list.Height, float32(visible)*rowHeight)
	canvas.HitWithOptions(projectsPanelID, list, "Project groups and files", gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	canvas.Clip(list)
	defer canvas.Unclip()
	fileCount := rows - len(panel.groups)
	for index := panel.offset; index < min(rows, panel.offset+visible); index++ {
		item := gfx.Rect{X: list.X, Y: list.Y + float32(index-panel.offset)*rowHeight, Width: list.Width, Height: rowHeight}
		if index <= panel.selected || index > panel.selected+fileCount {
			groupIndex := index
			if index > panel.selected {
				groupIndex -= fileCount
			}
			group := panel.groups[groupIndex]
			id := projectRows.At(groupIndex)
			selected := groupIndex == panel.selected
			if selected || canvas.Hovered() == id {
				canvas.Fill(item, palette.selectedRow)
			}
			icon := rightIcon
			if selected {
				icon = downIcon
			}
			canvas.DrawIcon(icon, gfx.Rect{X: item.X + 5, Y: item.Y + 6, Width: 12, Height: 12}, palette.muted)
			state.interfaceLabel(canvas, group.Name, gfx.Rect{X: item.X + 22, Y: item.Y, Width: item.Width - 26, Height: item.Height}, palette.text, 11)
			canvas.HitWithOptions(id, item, group.Name, gfx.HitOptions{Role: gfx.RoleButton, Selected: selected, Expanded: selected})
			if canvas.Clicked(id) {
				panel.selected = groupIndex
				panel.offset = min(panel.offset, groupIndex)
				return
			}
			continue
		}
		fileIndex := index - panel.selected - 1
		path := panel.groups[panel.selected].Files[fileIndex]
		name := filepath.Base(path)
		id := projectRows.At(len(panel.groups) + fileIndex)
		remove := gfx.Rect{X: item.X + item.Width - 23, Y: item.Y + 3, Width: 19, Height: 18}
		item.X, item.Width = item.X+19, item.Width-43
		if canvas.Hovered() == id || canvas.FocusVisible() == id {
			canvas.Fill(item, palette.hover)
		}
		state.interfaceLabel(canvas, name, item.Inset(gfx.Insets{Left: 4, Right: 3}), palette.text, 11)
		canvas.HitWithOptions(id, item, "Open "+name, gfx.HitOptions{Role: gfx.RoleButton, Hint: path})
		if canvas.Clicked(id) {
			if state.openDocumentPaths([]string{path}) {
				state.focusEditorOnTabClick(canvas)
			}
			return
		}
		canvas.DrawIcon(closeIcon, remove.Inset(gfx.All(3)), palette.muted)
		if hitButton(canvas, projectRemoveRows.At(fileIndex), remove, "Remove reference to "+name) {
			groups := slices.Clone(panel.groups)
			group := &groups[panel.selected]
			group.Files = slices.Delete(slices.Clone(group.Files), fileIndex, fileIndex+1)
			if state.updateProjects(groups) == nil {
				state.notice = "Reference removed; file unchanged"
			}
			return
		}
	}
}
