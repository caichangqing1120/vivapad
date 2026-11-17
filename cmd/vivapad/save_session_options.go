package main

import (
	"errors"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func (state *model) openSaveSessionOptions() {
	if _, err := state.nppSessionStateWithFolders(false); err != nil {
		state.notice = err.Error()
		return
	}
	if state.saveSessionPicker != nil {
		selection, err := state.saveSessionPicker(
			app.SaveFileOptions{Title: "Save Session", SuggestedName: "session.xml", DefaultExtension: "xml"},
			app.SaveFileCheckbox{Label: "Save Folder as Workspace", Checked: state.folderVisible},
		)
		if err == nil {
			state.saveNPPSessionNative(selection)
			return
		}
		if errors.Is(err, app.ErrCanceled) {
			return
		}
		if !errors.Is(err, app.ErrUnsupported) && !errors.Is(err, app.ErrNoDriverResponse) {
			state.notice = err.Error()
			return
		}
	}
	state.saveSessionOptionsOpen = true
	state.saveSessionIncludeWorkspace = state.folderVisible
	state.saveSessionError = ""
}

func (state *model) paintSaveSessionOptions(canvas *gfx.Canvas, size gfx.Size) {
	canvas.SetEscapeTarget(saveSessionCancelID)
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Save Session options", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width, height := min(float32(420), size.Width-32), float32(180)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	smallLabel(canvas, "Save Session", gfx.Rect{X: x + 20, Y: y + 16, Width: width - 40, Height: 27}, palette.text, 15)
	canvas.Fill(gfx.Rect{X: x + 1, Y: y + 52, Width: width - 2, Height: 1}, palette.border)
	box := gfx.Rect{X: x + 22, Y: y + 71, Width: 17, Height: 17}
	canvas.Border(box, 1, palette.border, palette.white)
	if state.saveSessionIncludeWorkspace {
		canvas.DrawIcon(checkIcon, box.Inset(gfx.All(2)), palette.blue)
	}
	toggle := gfx.Rect{X: x + 20, Y: y + 65, Width: width - 40, Height: 29}
	smallLabel(canvas, "Save Folder as Workspace", gfx.Rect{X: x + 48, Y: toggle.Y, Width: width - 70, Height: toggle.Height}, palette.text, 11)
	canvas.HitWithOptions(saveSessionWorkspaceID, toggle, "Save Folder as Workspace", gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: state.saveSessionIncludeWorkspace, Disabled: !state.folderVisible})
	if canvas.Clicked(saveSessionWorkspaceID) && state.folderVisible {
		state.saveSessionIncludeWorkspace = !state.saveSessionIncludeWorkspace
		state.saveSessionError = ""
	}
	if state.saveSessionError != "" {
		message := canvas.FitLine(state.saveSessionError, 11, gfx.WeightRegular, width-40)
		smallLabel(canvas, message, gfx.Rect{X: x + 20, Y: y + 105, Width: width - 40, Height: 18}, palette.blue, 11)
	}
	canvas.Fill(gfx.Rect{X: x + 1, Y: y + height - 52, Width: width - 2, Height: 1}, palette.border)
	for _, button := range []struct {
		id     gfx.ID
		label  string
		offset float32
	}{{saveSessionConfirmID, "Save...", 199}, {saveSessionCancelID, "Cancel", 101}} {
		bounds := gfx.Rect{X: x + width - button.offset, Y: y + height - 41, Width: 86, Height: 28}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, gfx.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			if button.id == saveSessionCancelID {
				state.saveSessionOptionsOpen = false
			} else {
				state.saveNPPSession(state.saveSessionIncludeWorkspace)
			}
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}
