package main

import (
	"path/filepath"
	"slices"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/recent"
)

// trashRequest is the confirmation dialog's state. Apply is one-shot and
// releases the source descriptor; a failed attempt also ends this confirmation,
// so moving again always needs a fresh Move to Recycle Bin command, a fresh
// identity capture and a fresh confirmation. Nothing can turn a refusal into a
// later move inside an already-open dialog.
type trashRequest struct {
	doc  *document.Document
	plan *document.TrashPlan
}

// dismissTrash closes the pending plan's source descriptor and clears the
// confirmation. Every cancel or discard path goes through it, so the descriptor
// TrashPlan holds from preparation is never leaked.
func (state *model) dismissTrash() {
	if request := state.pendingTrash; request != nil {
		request.closePlan()
		state.pendingTrash = nil
	}
}

func (request *trashRequest) closePlan() {
	if request == nil || request.plan == nil {
		return
	}
	request.plan.Close()
	request.plan = nil
}

func (state *model) requestTrash() {
	doc := state.space.Current()
	if doc.Path == "" {
		state.notice = "Save the document before moving it to the Recycle Bin"
		return
	}
	if state.isPrivateDataPath(doc.Path) {
		state.notice = "Cannot move an internal VivaPad data file to the Recycle Bin"
		return
	}
	plan, err := doc.PrepareTrash()
	if err != nil {
		state.notice = "Move to Recycle Bin: " + err.Error()
		return
	}
	state.pendingTrash = &trashRequest{doc: doc, plan: plan}
	state.notice = ""
}

func (state *model) confirmTrash() {
	request := state.pendingTrash
	if request == nil {
		return
	}
	index := state.documentIndex(request.doc)
	if index < 0 {
		state.dismissTrash()
		state.notice = "Document closed before it could be moved to the Recycle Bin"
		return
	}
	path := request.doc.Path
	plan := request.plan
	if plan == nil {
		state.dismissTrash()
		state.notice = "Move to Recycle Bin failed: confirmation expired, start again"
		return
	}
	request.plan = nil
	if err := plan.Apply(state.trashPath); err != nil {
		// A refusal or a failed backend request ends the confirmation: it must
		// never be possible to retry inside the same dialog and move a file that
		// replaced the one this dialog verified.
		state.dismissTrash()
		state.notice = "Move to Recycle Bin failed: " + err.Error()
		return
	}
	state.pendingTrash = nil
	if err := state.space.Close(index, true); err != nil {
		state.notice = "File moved to the Recycle Bin, but the tab could not close: " + err.Error()
		return
	}
	state.releaseDocument(request.doc)
	state.rebindClosedDocument(request.doc)
	state.recentEntries = slices.DeleteFunc(state.recentEntries, func(entry recent.Entry) bool { return entry.Path == path })
	state.closedHistory = slices.DeleteFunc(state.closedHistory, func(entry recent.Entry) bool { return entry.Path == path })
	var recentError error
	if state.recentPath != "" {
		recentError = recent.Save(state.recentPath, state.recentEntries)
	}
	closedError := state.saveClosedHistory()
	if recentError != nil {
		state.notice = "File moved, but recent history was not saved: " + recentError.Error()
		if closedError != nil {
			state.notice += "; closed history: " + closedError.Error()
		}
		return
	}
	if closedError != nil {
		state.notice = "File moved, but closed history was not saved: " + closedError.Error()
		return
	}
	state.notice = "Moved to Recycle Bin " + filepath.Base(path)
}

func (state *model) paintTrashDialog(canvas *gfx.Canvas, size gfx.Size) {
	palette := state.palette()
	request := state.pendingTrash
	if request == nil {
		return
	}
	canvas.SetEscapeTarget(trashCancelID)
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Move to Recycle Bin", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width, height := min(float32(440), size.Width-32), float32(184)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.popup)
	smallLabel(canvas, "Move to Recycle Bin?", gfx.Rect{X: x + 20, Y: y + 16, Width: width - 40, Height: 28}, palette.text, 15)
	name := canvas.FitLine(request.doc.Name, 13, gfx.WeightRegular, width-40)
	smallLabel(canvas, name, gfx.Rect{X: x + 20, Y: y + 52, Width: width - 40, Height: 22}, palette.text, 13)
	directory := canvas.FitLine(filepath.Dir(request.doc.Path), 11, gfx.WeightRegular, width-40)
	smallLabel(canvas, directory, gfx.Rect{X: x + 20, Y: y + 77, Width: width - 40, Height: 19}, palette.muted, 11)
	message := "The file will be moved to the system Trash."
	if request.doc.Dirty() {
		message = "Unsaved changes in this tab will be lost."
	}
	smallLabel(canvas, message, gfx.Rect{X: x + 20, Y: y + 107, Width: width - 40, Height: 22}, palette.accent, 11)
	canvas.Fill(gfx.Rect{X: x + 1, Y: y + height - 51, Width: width - 2, Height: 1}, palette.border)
	for _, button := range []struct {
		id     gfx.ID
		label  string
		bounds gfx.Rect
	}{
		{trashConfirmID, "Move to Trash", gfx.Rect{X: x + width - 240, Y: y + height - 42, Width: 122, Height: 29}},
		{trashCancelID, "Cancel", gfx.Rect{X: x + width - 104, Y: y + height - 42, Width: 86, Height: 29}},
	} {
		canvas.Border(button.bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, button.bounds, gfx.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, button.id, button.bounds, button.label) {
			if button.id == trashCancelID {
				state.dismissTrash()
			} else {
				state.confirmTrash()
			}
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}
