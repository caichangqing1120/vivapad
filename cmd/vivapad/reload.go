package main

import (
	"fmt"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func (state *model) requestReload() {
	index := state.space.Active
	doc := state.space.Current()
	if doc.Path == "" {
		state.notice = "Save the document before reloading from disk"
		return
	}
	if doc.Dirty() {
		state.pendingReload = index
		return
	}
	state.reloadDocument(index, false)
}

func (state *model) cancelReload() {
	state.pendingReload = -1
	state.focusViewRequested, state.focusViewArmed = true, false
	state.invalidator.Invalidate()
}

// reloadedNotice composes the reload notice from the replacement name and the
// change-history observation it already produced. The error is carried verbatim
// so it is never lost behind a bare success message.
func reloadedNotice(name string, historyErr error) string {
	notice := "Reloaded " + name
	if historyErr != nil {
		notice += "; " + historyErr.Error()
	}
	return notice
}

// reloadDocument reloads one tab and reports an unavailable replacement history
// through state.notice. It returns the replacement document, or nil when the
// reload failed.
func (state *model) reloadDocument(index int, discard bool) *document.Document {
	doc, _ := state.reloadDocumentWithHistory(index, discard)
	return doc
}

// reloadDocumentWithHistory is reloadDocument plus the replacement source's
// explicit change-history error. Callers that compose their own notice (the file
// monitor) must carry this returned error instead of re-deriving it: the
// consumer is stateful, so a second ChangeHistory call would sync again rather
// than report the same lifecycle observation.
func (state *model) reloadDocumentWithHistory(index int, discard bool) (*document.Document, error) {
	previous := state.space.Documents[index]
	state.cancelDocumentComposition(previous)
	previousValue := previous.Editor.String()
	previousRectangles := state.rectangles
	readOnly := state.readOnly[previous]
	caret := previous.Editor.Caret()
	language, selected := state.languages[previous]
	var oldErr error
	if discard {
		_, oldErr = state.space.ReloadDiscard(index)
	} else {
		_, oldErr = state.space.Reload(index)
	}
	if oldErr != nil {
		state.notice = "Reload failed: " + oldErr.Error()
		return nil, nil
	}
	current := state.space.Documents[index]
	state.rebindBookmarksOnReload(previous, current)
	if readOnly {
		state.readOnly[current] = true
	}
	current.Editor.MoveTo(min(caret, current.Editor.RuneCount()), false)
	state.replacePaneDocument(previous, current)
	if editor := state.editors[previous]; editor != nil {
		state.editors[current] = editor
	}
	if editor := state.secondEditors[previous]; editor != nil {
		state.secondEditors[current] = editor
	}
	for pane, anchor := range state.paneTabAnchor {
		if anchor == previous {
			state.paneTabAnchor[pane] = current
		}
	}
	if watcher := state.monitors[previous]; watcher != nil {
		delete(state.monitors, previous)
		state.monitors[current] = watcher
	}
	if color := state.tabColors[previous]; color > 0 {
		state.tabColors[current] = color
	}
	state.releaseDocument(previous)
	for pane, selection := range previousRectangles {
		if selection.doc != previous || state.paneDocument(pane) != current {
			continue
		}
		if rebased, ok := rebaseRectangleOnReload(selection, previousValue, current.Editor.String()); ok {
			rebased.doc = current
			state.rectangles[pane] = rebased
			if pane == state.activePane && rebased.active && previousValue != current.Editor.String() {
				current.Editor.MoveTo(rebased.caret, false)
			}
		}
	}
	if selected {
		state.languages[current] = language
	}
	// The replacement is a new bound source; consume it once so a constructor
	// failure surfaces here instead of as an empty marker history. The error is
	// both composed into this notice and returned for callers that compose their
	// own notice.
	_, historyErr := current.ChangeHistory()
	state.notice = reloadedNotice(current.Name, historyErr)
	return current, historyErr
}

func (state *model) paintReloadDialog(canvas *gfx.Canvas, size gfx.Size) {
	if state.pendingReload >= len(state.space.Documents) {
		state.pendingReload = -1
		return
	}
	canvas.SetEscapeTarget(reloadCancelID)
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Reload confirmation", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width := float32(388)
	x, y := (size.Width-width)/2, (size.Height-158)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: 158}, 4, 1, palette.border, palette.white)
	name := state.space.Documents[state.pendingReload].Name
	title := canvas.FitLine(fmt.Sprintf("Reload %s from disk?", name), 15, gfx.WeightRegular, width-40)
	smallLabel(canvas, title, gfx.Rect{X: x + 20, Y: y + 18, Width: width - 40, Height: 30}, palette.text, 15)
	smallLabel(canvas, "Unsaved changes in this tab will be discarded.", gfx.Rect{X: x + 20, Y: y + 56, Width: width - 40, Height: 25}, palette.muted, 12)
	for _, item := range []struct {
		id    gfx.ID
		label string
		x     float32
	}{
		{reloadConfirmID, "Reload", x + 188},
		{reloadCancelID, "Cancel", x + 284},
	} {
		bounds := gfx.Rect{X: item.x, Y: y + 105, Width: 80, Height: 30}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(item.label, bounds, gfx.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, item.id, bounds, item.label) {
			index := state.pendingReload
			if item.id == reloadConfirmID {
				state.pendingReload = -1
				state.reloadDocument(index, true)
			} else {
				state.cancelReload()
			}
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}
