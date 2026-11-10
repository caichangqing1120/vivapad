package main

import (
	"errors"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/hashops"
)

type hashDialogMode uint8

const (
	hashFromText hashDialogMode = iota + 1
	hashFromFiles
)

var hashCommands = [...]string{"Generate...", "Generate from files...", "Generate from selection into clipboard"}

func hashAlgorithmForName(name string) hashops.Algorithm {
	switch name {
	case "MD5":
		return hashops.MD5
	case "SHA-1":
		return hashops.SHA1
	case "SHA-256":
		return hashops.SHA256
	case "SHA-512":
		return hashops.SHA512
	}
	return 0
}

func (state *model) hashSelectionToClipboard(doc *document.Document) {
	selection := doc.Editor.Selection()
	if selection.Start == selection.End {
		state.notice = "Select text to hash"
		return
	}
	value := []rune(doc.Editor.String())[selection.Start:selection.End]
	result, err := hashops.Digest(state.hashAlgorithm, []byte(string(value)))
	if err == nil {
		err = app.SetClipboardText(result)
	}
	if err != nil {
		state.notice = "Hash copy failed: " + err.Error()
		return
	}
	state.notice = state.hashAlgorithm.String() + " copied"
}

func (state *model) openHashDialog(mode hashDialogMode) {
	if state.hashAlgorithm == 0 {
		state.notice = "Choose a hash algorithm"
		return
	}
	state.hashDialogOpen, state.hashMode = true, mode
	state.hashInput = text.NewEditor("")
	state.hashInputLayout, state.hashResultLayout = components.TextareaLayout{}, components.TextareaLayout{}
	state.hashResult, state.hashError = "", ""
	state.hashEachLine, state.hashFocus = false, mode == hashFromText
}

func (state *model) closeHashDialog() {
	state.hashDialogOpen, state.hashInput, state.hashResult, state.hashError, state.hashFocus = false, nil, "", "", false
}

func (state *model) chooseHashFiles() {
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: "Choose files to hash", AllowMultiple: true})
	if errors.Is(err, app.ErrCanceled) {
		return
	}
	if err == nil {
		state.hashResult, err = hashops.Files(state.hashAlgorithm, paths)
	}
	if err != nil {
		state.hashResult, state.hashError = "", err.Error()
	} else {
		state.hashError = ""
	}
}

func (state *model) copyHashResult() {
	if state.hashResult == "" {
		return
	}
	if err := app.SetClipboardText(state.hashResult); err != nil {
		state.hashError = err.Error()
		return
	}
	state.hashError = ""
}

func (state *model) paintHashMenu(canvas *ui.Canvas, size ui.Size, parentLeft, parentRight float32) {
	width := min(float32(252), size.Width-8)
	height := float32(len(hashCommands))*27 + 8
	x := parentRight
	if x+width > size.Width-2 {
		x = max(2, parentLeft-width)
	}
	y := min(menuHeight+4+float32(state.hashMenuIndex)*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range hashCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := hashScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(command, 11, ui.WeightRegular, item.Width-18)
		smallLabel(canvas, label, ui.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, ui.HitOptions{Role: ui.RoleMenuItem, Hint: command})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(hashCommands[selected])
	}
}

func (state *model) paintHashDialog(canvas *ui.Canvas, size ui.Size) {
	if state.hashMode == hashFromText {
		state.hashResult, _ = hashops.Text(state.hashAlgorithm, state.hashInput.String(), state.hashEachLine)
	}
	canvas.SetEscapeTarget(hashCloseID)
	canvas.PushLayer(3)
	canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, ui.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, ui.Rect{Width: size.Width, Height: size.Height}, "Hash tool", ui.HitOptions{Role: ui.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width := min(float32(500), size.Width-32)
	height := min(float32(310), size.Height-24)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(ui.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	smallLabel(canvas, "Generate "+state.hashAlgorithm.String(), ui.Rect{X: x + 20, Y: y + 13, Width: width - 40, Height: 28}, palette.text, 15)
	theme := ui.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 28, 12, 2
	theme.Border, theme.Ring = palette.border, palette.blue
	if state.hashMode == hashFromText {
		input := state.hashInput.String()
		selection := state.hashInput.Selection()
		components.PaintTextarea(canvas, theme, ui.Rect{X: x + 20, Y: y + 46, Width: width - 40, Height: 83}, components.TextareaSnapshot{
			ID: hashInputID, Label: "Text to hash", Value: input, Placeholder: "Text", HideEditorText: true, Layout: &state.hashInputLayout,
			ShowCaret: true, Caret: state.hashInput.ByteOffset(state.hashInput.Caret()),
			SelectionStart: state.hashInput.ByteOffset(selection.Start), SelectionEnd: state.hashInput.ByteOffset(selection.End),
		})
		if state.hashFocus {
			canvas.RequestFocus(hashInputID)
			state.hashFocus = false
		}
		checkbox := ui.Rect{X: x + 20, Y: y + 135, Width: width - 40, Height: 23}
		box := ui.Rect{X: checkbox.X, Y: checkbox.Y + 3, Width: 17, Height: 17}
		canvas.Border(box, 1, palette.border, palette.white)
		if state.hashEachLine {
			canvas.DrawIcon(checkIcon, box.Inset(ui.All(2)), palette.blue)
		}
		smallLabel(canvas, "Hash each line", ui.Rect{X: checkbox.X + 26, Y: checkbox.Y, Width: checkbox.Width - 26, Height: 23}, palette.text, 11)
		canvas.HitWithOptions(hashEachLineID, checkbox, "Hash each line", ui.HitOptions{Role: ui.RoleCheckbox, Checked: state.hashEachLine})
		if canvas.Clicked(hashEachLineID) {
			state.hashEachLine = !state.hashEachLine
			state.hashResult, _ = hashops.Text(state.hashAlgorithm, input, state.hashEachLine)
		}
	} else {
		choose := ui.Rect{X: x + 20, Y: y + 52, Width: 146, Height: 28}
		canvas.Border(choose, 1, palette.border, palette.toolbar)
		canvas.LabelCenter("Choose files...", choose, ui.TextStyle{Size: 11, Color: palette.text})
		if hitButton(canvas, hashBrowseID, choose, "Choose files to hash") {
			state.chooseHashFiles()
		}
	}
	resultY := y + 163
	if state.hashMode == hashFromFiles {
		resultY = y + 90
	}
	components.PaintTextarea(canvas, theme, ui.Rect{X: x + 20, Y: resultY, Width: width - 40, Height: y + height - 60 - resultY}, components.TextareaSnapshot{
		ID: hashResultID, Label: "Hash result", Value: state.hashResult, ReadOnly: true, Placeholder: "Result", Layout: &state.hashResultLayout,
	})
	if state.hashError != "" {
		message := canvas.FitLine(state.hashError, 11, ui.WeightRegular, width-40)
		smallLabel(canvas, message, ui.Rect{X: x + 20, Y: y + height - 52, Width: width - 40, Height: 18}, palette.blue, 11)
	}
	for _, button := range []struct {
		id    ui.ID
		label string
		x     float32
	}{
		{hashCopyID, "Copy", x + width - 190},
		{hashCloseID, "Close", x + width - 100},
	} {
		bounds := ui.Rect{X: button.x, Y: y + height - 43, Width: 80, Height: 28}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, ui.TextStyle{Size: 11, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			if button.id == hashCloseID {
				state.closeHashDialog()
			} else {
				state.copyHashResult()
			}
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}
