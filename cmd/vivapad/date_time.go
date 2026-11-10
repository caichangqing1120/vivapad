package main

import (
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"golang.org/x/text/language"
	"vivapad/internal/datetime"
	"vivapad/internal/document"
)

var insertDateCommands = [...]string{"Date Time (short)", "Date Time (long)", "Date Time (customized)"}

func (state *model) insertDateTime(doc *document.Document, kind datetime.Kind) {
	clock := state.clock
	if clock == nil {
		clock = time.Now
	}
	locale := language.Und
	if preferred := app.PreferredLanguages(); len(preferred) > 0 {
		locale = language.Make(preferred[0])
	}
	value, err := datetime.Format(clock(), kind, state.dateTimeFormat, state.dateTimeReverse, locale)
	if err != nil {
		state.notice = "Date time format: " + err.Error()
		return
	}
	state.editWithBookmarkAnchor(doc, func() { doc.Editor.ReplaceRange(doc.Editor.Selection(), value) })
	state.notice = "Inserted date and time"
}

func (state *model) openDateTimeDialog() {
	state.dateTimeDraft = text.NewEditor(state.dateTimeFormat)
	state.dateTimeDraft.SetSelection(text.Range{Start: 0, End: state.dateTimeDraft.RuneCount()})
	state.dateTimeDraftReverse = state.dateTimeReverse
	state.dateTimeError = ""
	state.dateTimeDialogOpen, state.dateTimeFocus = true, true
}

func (state *model) closeDateTimeDialog() {
	state.dateTimeDialogOpen, state.dateTimeDraft, state.dateTimeFocus, state.dateTimeError = false, nil, false, ""
}

func (state *model) applyDateTimeSettings() {
	if !state.dateTimeDialogOpen || state.dateTimeDraft == nil {
		return
	}
	pattern := state.dateTimeDraft.String()
	if err := datetime.ValidatePattern(pattern); err != nil {
		state.dateTimeError = err.Error()
		return
	}
	previousFormat, previousOrder := state.dateTimeFormat, state.dateTimeReverse
	state.dateTimeFormat, state.dateTimeReverse = pattern, state.dateTimeDraftReverse
	if err := state.savePreferences(); err != nil {
		state.dateTimeFormat, state.dateTimeReverse = previousFormat, previousOrder
		state.dateTimeError = err.Error()
		return
	}
	state.closeDateTimeDialog()
	state.notice = "Date time format saved"
}

func (state *model) paintInsertDateMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(230), size.Width-parentRight-4)
	height := float32(len(insertDateCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+19*17, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range insertDateCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := insertDateScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(command, 11, ui.WeightRegular, item.Width-18)
		smallLabel(canvas, label, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, ui.HitOptions{Role: ui.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(insertDateCommands[selected])
	}
}

func (state *model) paintDateTimeDialog(canvas *ui.Canvas, size ui.Size) {
	canvas.SetEscapeTarget(dateTimeCancelID)
	canvas.PushLayer(3)
	canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, ui.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, ui.Rect{Width: size.Width, Height: size.Height}, "Date time settings", ui.HitOptions{Role: ui.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width := min(float32(420), size.Width-32)
	x, y := (size.Width-width)/2, (size.Height-208)/2
	canvas.RoundedBorder(ui.Rect{X: x, Y: y, Width: width, Height: 208}, 4, 1, palette.border, palette.white)
	smallLabel(canvas, "Date Time Format", ui.Rect{X: x + 20, Y: y + 15, Width: width - 40, Height: 26}, palette.text, 15)
	theme := ui.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 30, 12, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	components.PaintInput(canvas, theme, ui.Rect{X: x + 20, Y: y + 48, Width: width - 40, Height: 30}, components.InputSnapshot{
		ID: dateTimeFieldID, Label: "Customized format", Value: state.dateTimeDraft.String(), PlatformEditing: canvas.Focused() == dateTimeFieldID,
	})
	if state.dateTimeFocus {
		canvas.RequestFocus(dateTimeFieldID)
		state.dateTimeFocus = false
	}
	toggle := ui.Rect{X: x + 20, Y: y + 88, Width: width - 40, Height: 24}
	box := ui.Rect{X: toggle.X, Y: toggle.Y + 3, Width: 17, Height: 17}
	canvas.Border(box, 1, palette.border, palette.white)
	if state.dateTimeDraftReverse {
		canvas.DrawIcon(checkIcon, box.Inset(ui.All(2)), palette.blue)
	}
	smallLabel(canvas, "Date before time (short and long)", ui.Rect{X: x + 46, Y: toggle.Y, Width: width - 68, Height: 24}, palette.text, 11)
	canvas.HitWithOptions(dateTimeReverseID, toggle, "Date before time", ui.HitOptions{Role: ui.RoleCheckbox, Checked: state.dateTimeDraftReverse})
	if canvas.Clicked(dateTimeReverseID) {
		state.dateTimeDraftReverse = !state.dateTimeDraftReverse
	}
	if state.dateTimeError != "" {
		message := canvas.FitLine(state.dateTimeError, 11, ui.WeightRegular, width-40)
		smallLabel(canvas, message, ui.Rect{X: x + 20, Y: y + 120, Width: width - 40, Height: 21}, palette.blue, 11)
	}
	for _, button := range []struct {
		id    ui.ID
		label string
		x     float32
	}{
		{dateTimeCancelID, "Cancel", x + width - 190},
		{dateTimeApplyID, "Save", x + width - 100},
	} {
		bounds := ui.Rect{X: button.x, Y: y + 158, Width: 80, Height: 30}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, ui.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			if button.id == dateTimeCancelID {
				state.closeDateTimeDialog()
			} else {
				state.applyDateTimeSettings()
			}
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}
