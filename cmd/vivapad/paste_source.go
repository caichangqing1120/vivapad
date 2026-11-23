package main

import (
	"bytes"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

const (
	pasteSpecialCommand    = "Paste Special"
	pasteHTMLCommand       = "Paste HTML Content"
	pasteRTFCommand        = "Paste RTF Content"
	maxClipboardSourceSize = 4 << 20
)

func isPasteSourceCommand(command string) bool {
	return command == pasteHTMLCommand || command == pasteRTFCommand
}

func (state *model) pasteClipboardSource(command string) {
	doc := state.paneDocument(state.activePane)
	if doc == nil || doc.Editor == nil {
		return
	}
	if state.isReadOnly(doc) {
		state.notice = "Read-only document"
		return
	}
	format := app.ClipboardHTMLSource
	if command == pasteRTFCommand {
		format = app.ClipboardRTFSource
	}
	source, err := app.ClipboardSource(format, maxClipboardSourceSize)
	if err != nil {
		state.notice = "Paste failed: " + err.Error()
		return
	}
	if len(source) == 0 || !utf8.Valid(source) || bytes.IndexByte(source, 0) >= 0 {
		state.notice = "Paste failed: clipboard source contains invalid text"
		return
	}
	state.cancelDocumentComposition(doc)
	input := state.documentTextInput(state.activePane)
	if input == nil || input.OnEvent == nil {
		return
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextPaste, Text: string(source)}); err != nil {
		state.notice = "Paste failed: " + err.Error()
	}
}

func (state *model) paintPasteSpecial(canvas *gfx.Canvas, size gfx.Size, parentRight, parentY float32) {
	commands := [...]string{pasteHTMLCommand, pasteRTFCommand}
	width := min(float32(220), size.Width-parentRight-4)
	height := float32(len(commands))*27 + 8
	y := min(max(menuHeight, parentY), max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: parentRight, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := ""
	for index, command := range commands {
		item := gfx.Rect{X: parentRight + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := pasteSpecialScope.At(index)
		if canvas.Hovered() == id || canvas.FocusVisible() == id {
			canvas.Fill(item, palette.hover)
		}
		label := canvas.FitLine(command, 11, gfx.WeightRegular, item.Width-18)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 9, Y: item.Y, Width: item.Width - 18, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem})
		if canvas.Clicked(id) {
			selected = command
		}
	}
	canvas.PopLayer()
	if selected != "" {
		state.command(selected)
	}
}
