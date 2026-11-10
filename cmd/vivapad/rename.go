package main

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func (state *model) requestRename() {
	doc := state.space.Current()
	if doc.Path == "" {
		state.renameDoc = doc
		state.renameField = text.NewEditor(doc.Name)
		state.renameField.SetSelection(text.Range{Start: 0, End: state.renameField.RuneCount()})
		state.renameFocus = true
		return
	}
	if state.isPrivateDataPath(doc.Path) {
		state.notice = "Cannot rename an internal VivaPad data file"
		return
	}
	extension := strings.TrimPrefix(filepath.Ext(doc.Name), ".")
	path, err := app.SaveFile(app.SaveFileOptions{Title: "Rename", SuggestedName: doc.Name, DefaultExtension: extension})
	if err != nil {
		if !errors.Is(err, app.ErrCanceled) {
			state.notice = err.Error()
		}
		return
	}
	plan, err := doc.PrepareRename(path)
	if err != nil {
		state.notice = "Rename failed: " + err.Error()
		return
	}
	if state.isPrivateDataPath(plan.Target) {
		state.notice = "Rename destination is an internal VivaPad data file"
		return
	}
	for _, other := range state.space.Documents {
		if other != doc && other.Path == plan.Target {
			state.notice = "Rename destination is already open in another tab"
			return
		}
	}
	if plan.Target == doc.Path {
		state.notice = "File name unchanged"
		return
	}
	if plan.TargetExists {
		state.renameSource = doc
		state.renamePlan = plan
		return
	}
	state.applyRenamePlan(plan, false)
}

func (state *model) applyRenamePlan(plan *document.RenamePlan, replace bool) {
	doc := state.renameSource
	if doc == nil {
		doc = state.space.Current()
	}
	oldPath := doc.Path
	if err := plan.Apply(replace); err != nil {
		state.notice = "Rename failed: " + err.Error()
		return
	}
	state.renamePlan, state.renameSource = nil, nil
	entries := state.recentEntries[:0]
	for _, entry := range state.recentEntries {
		if entry.Path != oldPath {
			entries = append(entries, entry)
		}
	}
	state.recentEntries = entries
	state.configureCode(doc)
	state.notice = "Renamed to " + doc.Name
	state.rememberRecent(doc)
}

func (state *model) applyUntitledRename() {
	doc := state.renameDoc
	if doc == nil || state.renameField == nil {
		return
	}
	name := strings.TrimSpace(state.renameField.String())
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "\\/:*?\"<>|\r\n\t") || len([]rune(name)) > 254 {
		state.notice = "Rename failed: enter a valid tab name"
		return
	}
	for _, other := range state.space.Documents {
		if other != doc && strings.EqualFold(other.Name, name) {
			state.notice = "Rename failed: name is already in use"
			return
		}
	}
	doc.Name = name
	state.configureCode(doc)
	state.renameDoc, state.renameField = nil, nil
	state.notice = "Renamed tab to " + name
}

func (state *model) cancelRename() {
	state.renameDoc, state.renameSource, state.renameField, state.renamePlan = nil, nil, nil, nil
	state.renameFocus = false
}

func (state *model) paintRenameDialog(canvas *ui.Canvas, size ui.Size) {
	canvas.SetEscapeTarget(renameCancelID)
	canvas.PushLayer(3)
	canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, ui.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, ui.Rect{Width: size.Width, Height: size.Height}, "Rename dialog", ui.HitOptions{Role: ui.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width := min(float32(388), size.Width-32)
	x, y := (size.Width-width)/2, (size.Height-158)/2
	canvas.RoundedBorder(ui.Rect{X: x, Y: y, Width: width, Height: 158}, 4, 1, palette.border, palette.white)
	if state.renameDoc != nil {
		smallLabel(canvas, "Rename Current Tab", ui.Rect{X: x + 20, Y: y + 16, Width: width - 40, Height: 30}, palette.text, 15)
		theme := ui.LightTheme()
		theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 28, 12, 2
		theme.Border, theme.Ring = palette.border, palette.blue
		components.PaintInput(canvas, theme, ui.Rect{X: x + 20, Y: y + 55, Width: width - 40, Height: 28}, components.InputSnapshot{
			ID: renameFieldID, Label: "New tab name", Value: state.renameField.String(), PlatformEditing: canvas.Focused() == renameFieldID,
		})
		if state.renameFocus {
			canvas.RequestFocus(renameFieldID)
			state.renameFocus = false
		}
	} else {
		smallLabel(canvas, "Replace existing file?", ui.Rect{X: x + 20, Y: y + 16, Width: width - 40, Height: 30}, palette.text, 15)
		name := canvas.FitLine(filepath.Base(state.renamePlan.Target), 13, ui.WeightRegular, width-40)
		smallLabel(canvas, name, ui.Rect{X: x + 20, Y: y + 50, Width: width - 40, Height: 24}, palette.text, 13)
		directory := canvas.FitLine(filepath.Dir(state.renamePlan.Target), 10, ui.WeightRegular, width-40)
		smallLabel(canvas, directory, ui.Rect{X: x + 20, Y: y + 75, Width: width - 40, Height: 20}, palette.muted, 10)
	}
	buttons := []struct {
		id    ui.ID
		label string
		x     float32
	}{
		{renameCancelID, "Cancel", x + width - 104},
		{renameApplyID, "Rename", x + width - 196},
	}
	if state.renamePlan != nil {
		buttons[1] = struct {
			id    ui.ID
			label string
			x     float32
		}{renameConfirmID, "Replace", x + width - 196}
	}
	for _, button := range buttons {
		bounds := ui.Rect{X: button.x, Y: y + 105, Width: 80, Height: 30}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, ui.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			switch button.id {
			case renameCancelID:
				state.cancelRename()
			case renameApplyID:
				state.applyUntitledRename()
			case renameConfirmID:
				state.applyRenamePlan(state.renamePlan, true)
			}
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}
