package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
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
	defer state.requestNativeChooserFocus()
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
	if !state.renameDestinationAvailable(doc, plan.Target) {
		return
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

func (state *model) renameDestinationAvailable(doc *document.Document, path string) bool {
	if state.isPrivateDataPath(path) {
		state.notice = "Rename destination is an internal VivaPad data file"
		return false
	}
	target, _ := os.Stat(path)
	for _, other := range state.space.Documents {
		if other != doc && other.Path != "" && destinationMatchesFile(path, target, other.Path) {
			state.notice = "Rename destination is already open in another tab"
			return false
		}
	}
	return true
}

func (state *model) applyRenamePlan(plan *document.RenamePlan, replace bool) {
	doc := state.renameSource
	if doc == nil {
		doc = state.space.Current()
	}
	if replace && !state.renameDestinationAvailable(doc, plan.Target) {
		return
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
	state.stopMonitorAfterPathChange(doc)
	state.rememberRecent(doc)
	if replace {
		state.requestNativeChooserFocus()
	}
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
	wasConfirmingReplace := state.renamePlan != nil
	state.renameDoc, state.renameSource, state.renameField, state.renamePlan = nil, nil, nil, nil
	state.renameFocus = false
	if wasConfirmingReplace {
		state.requestNativeChooserFocus()
	}
}

func (state *model) paintRenameDialog(canvas *gfx.Canvas, size gfx.Size) {
	canvas.SetEscapeTarget(renameCancelID)
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Rename dialog", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width := min(float32(388), size.Width-32)
	x, y := (size.Width-width)/2, (size.Height-158)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: 158}, 4, 1, palette.border, palette.white)
	if state.renameDoc != nil {
		smallLabel(canvas, "Rename Current Tab", gfx.Rect{X: x + 20, Y: y + 16, Width: width - 40, Height: 30}, palette.text, 15)
		theme := gfx.LightTheme()
		theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 28, 12, 2
		theme.Border, theme.Ring = palette.border, palette.blue
		components.PaintInput(canvas, theme, gfx.Rect{X: x + 20, Y: y + 55, Width: width - 40, Height: 28}, components.InputSnapshot{
			ID: renameFieldID, Label: "New tab name", Value: state.renameField.String(), PlatformEditing: canvas.Focused() == renameFieldID,
		})
		if state.renameFocus {
			canvas.RequestFocus(renameFieldID)
			state.renameFocus = false
		}
	} else {
		smallLabel(canvas, "Replace existing file?", gfx.Rect{X: x + 20, Y: y + 16, Width: width - 40, Height: 30}, palette.text, 15)
		name := canvas.FitLine(filepath.Base(state.renamePlan.Target), 13, gfx.WeightRegular, width-40)
		smallLabel(canvas, name, gfx.Rect{X: x + 20, Y: y + 50, Width: width - 40, Height: 24}, palette.text, 13)
		directory := canvas.FitLine(filepath.Dir(state.renamePlan.Target), 10, gfx.WeightRegular, width-40)
		smallLabel(canvas, directory, gfx.Rect{X: x + 20, Y: y + 75, Width: width - 40, Height: 20}, palette.muted, 10)
	}
	buttons := []struct {
		id    gfx.ID
		label string
		x     float32
	}{
		{renameCancelID, "Cancel", x + width - 104},
		{renameApplyID, "Rename", x + width - 196},
	}
	if state.renamePlan != nil {
		buttons[1] = struct {
			id    gfx.ID
			label string
			x     float32
		}{renameConfirmID, "Replace", x + width - 196}
	}
	for _, button := range buttons {
		bounds := gfx.Rect{X: button.x, Y: y + 105, Width: 80, Height: 30}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, gfx.TextStyle{Size: 12, Color: palette.text})
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
