package main

import (
	"math"
	"slices"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

var (
	statusGoToID      = tree.ID(page, "status-go-to")
	statusEncodingID  = tree.ID(page, "status-encoding")
	statusEOLID       = tree.ID(page, "status-eol")
	statusLanguageID  = tree.ID(page, "status-language")
	statusDismissID   = tree.ID(page, "status-dismiss")
	statusMenuID      = tree.ID(page, "status-menu")
	statusPreviousID  = tree.ID(page, "status-previous")
	statusNextID      = tree.ID(page, "status-next")
	statusChoiceScope = tree.Sub(page, tree.Name("status-choices"))
)

type statusPopup struct {
	kind            gfx.ID
	anchor          gfx.Rect
	doc             *document.Document
	offset, visible int
}

var quickEncodings = []string{"UTF-8", "UTF-8 BOM", "UTF-16 LE", "UTF-16 BE", "GB18030", "Windows-1252"}

var quickLanguages = func() []string {
	commands := slices.Clone(menuCommands[5])
	for _, language := range languageCommands {
		if !slices.Contains(commands, language.Label) {
			commands = append(commands, language.Label)
		}
	}
	return commands
}()

func (popup statusPopup) choices() ([]string, string) {
	switch popup.kind {
	case statusEOLID:
		return eolCommands[:], "Line endings"
	case statusEncodingID:
		return quickEncodings, "Encodings"
	case statusLanguageID:
		return quickLanguages, "Languages"
	}
	return nil, ""
}

func scrollListStep(event gfx.ScrollEvent) int {
	delta := float64(event.StepY(23))
	return int(math.Copysign(math.Ceil(math.Abs(delta)), delta))
}

func (state *model) paintStatusAction(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, label string) {
	palette := state.palette()
	hint := ""
	if !id.IsZero() {
		if state.statusPopup.kind == id {
			state.statusPopup.anchor = bounds
		}
		switch id {
		case statusGoToID:
			hint = "Go to Line"
		case statusEOLID:
			hint = "Change line endings"
		case statusEncodingID:
			hint = "Change encoding"
		case statusLanguageID:
			hint = "Change language"
		}
		pill := bounds.Inset(gfx.Insets{Top: 2, Right: 3, Bottom: 3, Left: 3})
		if canvas.Hovered() == id || state.statusPopup.kind == id {
			canvas.Rounded(pill, 4, palette.toolbarHover)
		}
		if canvas.FocusVisible() == id {
			canvas.RoundedBorder(pill, 4, 1, palette.toolbarFocus, gfx.Color{})
		}
		canvas.HitWithOptions(id, bounds, hint+": "+label, gfx.HitOptions{Role: gfx.RoleButton, Hint: label, Expanded: state.statusPopup.kind == id})
		if canvas.Clicked(id) {
			state.useStatusAction(canvas, id, bounds)
		}
	}
	canvas.Fill(gfx.Rect{X: bounds.X, Y: bounds.Y + 6, Width: 1, Height: 10}, palette.toolbarSeparator)
	textBounds := gfx.Rect{X: bounds.X + 7, Y: bounds.Y, Width: bounds.Width - 9, Height: 21}
	state.interfaceLabel(canvas, label, textBounds, palette.text, 11)
}

func (state *model) useStatusAction(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect) {
	if id == statusGoToID {
		state.statusPopup = statusPopup{}
		state.command("Go to Line...")
		canvas.RequestFocusOnce(goToFieldID)
		return
	}
	doc := state.space.Current()
	if (id == statusEOLID || id == statusEncodingID) && state.isReadOnly(doc) {
		state.statusPopup = statusPopup{}
		state.notice = "Read-only document"
		if doc.LargeFile() != nil {
			state.notice = document.ErrLargeFileReadOnly.Error()
		}
		canvas.RequestFocusOnce(state.commandEditorFocusID())
		state.invalidator.Invalidate()
		return
	}
	if state.statusPopup.kind == id {
		state.statusPopup = statusPopup{}
		canvas.RequestFocusOnce(state.commandEditorFocusID())
		return
	}
	state.statusPopup = statusPopup{kind: id, anchor: bounds, doc: doc}
	canvas.RequestFocusOnce(statusChoiceScope.At(0))
}

func (state *model) paintStatusPopup(canvas *gfx.Canvas, size gfx.Size) {
	popup := &state.statusPopup
	items, title := popup.choices()
	if popup.kind == statusLanguageID && len(state.udls) > 0 {
		items = make([]string, 0, len(quickLanguages)+len(state.udls))
		for _, definition := range state.udls {
			items = append(items, udlLabelPrefix+definition.Name)
		}
		items = append(items, quickLanguages...)
	}
	if len(items) == 0 || popup.doc != state.space.Current() || state.editorCommandFocusBlocked() || state.distractionFree || state.postIt.active {
		*popup = statusPopup{}
		return
	}
	const rowHeight = 23
	visible := min(len(items), max(1, int((min(size.Height-statusHeight-12, 316)-8)/rowHeight)))
	controls := float32(0)
	if visible < len(items) {
		controls = 2 * rowHeight
		visible = max(1, visible-2)
	}
	popup.visible = visible
	maximum := len(items) - visible
	popup.offset = min(max(0, popup.offset), maximum)
	width, height := min(float32(204), size.Width-8), float32(visible)*rowHeight+controls+8
	x, y := min(max(4, popup.anchor.X+popup.anchor.Width-width), size.Width-width-4), size.Height-statusHeight-height-3
	palette := state.palette()
	canvas.SetEscapeTarget(statusDismissID)
	canvas.PushLayer(3)
	canvas.HitWithOptions(statusDismissID, gfx.Rect{Width: size.Width, Height: size.Height - statusHeight}, "Dismiss status menu", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.popup)
	canvas.HitWithOptions(statusMenuID, panel, title, gfx.HitOptions{Role: gfx.RoleMenu, SkipFocus: true})
	if controls != 0 {
		for _, control := range []struct {
			id    gfx.ID
			label string
			icon  gfx.Icon
			y     float32
			step  int
		}{{statusPreviousID, "Previous choices", upIcon, y + 4, -1}, {statusNextID, "More choices", downIcon, y + height - rowHeight - 4, 1}} {
			bounds := gfx.Rect{X: x + 4, Y: control.y, Width: width - 8, Height: rowHeight}
			if canvas.Hovered() == control.id {
				canvas.Fill(bounds, palette.hover)
			}
			canvas.DrawIcon(control.icon, gfx.Rect{X: x + (width-14)/2, Y: bounds.Y + 4, Width: 14, Height: 14}, palette.muted)
			if hitButton(canvas, control.id, bounds, control.label) {
				popup.offset = min(max(0, popup.offset+control.step), maximum)
			}
		}
	}
	selected := ""
	for row := range visible {
		command := items[popup.offset+row]
		id := statusChoiceScope.At(row)
		bounds := gfx.Rect{X: x + 4, Y: y + 4 + controls/2 + float32(row)*rowHeight, Width: width - 8, Height: rowHeight}
		if canvas.Hovered() == id || canvas.FocusVisible() == id {
			canvas.Fill(bounds, palette.hover)
		}
		checked := state.menuSelected(command)
		if checked {
			canvas.DrawIcon(checkIcon, gfx.Rect{X: x + 9, Y: bounds.Y + 4, Width: 14, Height: 14}, palette.accent)
		}
		smallLabel(canvas, canvas.FitLine(command, 11, gfx.WeightRegular, width-36), gfx.Rect{X: x + 27, Y: bounds.Y, Width: width - 33, Height: rowHeight}, palette.text, 11)
		canvas.HitWithOptions(id, bounds, command, gfx.HitOptions{Role: gfx.RoleMenuItem, Checked: checked})
		if canvas.Clicked(id) {
			selected = command
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
	if selected != "" || canvas.Clicked(statusDismissID) {
		*popup = statusPopup{}
		if selected != "" {
			state.command(selected)
		}
		canvas.RequestFocusOnce(state.commandEditorFocusID())
	}
}
