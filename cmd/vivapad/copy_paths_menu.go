package main

import (
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

var copyPathsCommands = [...]string{
	"Copy Current Full File path", "Copy Current Filename", "Copy Current Dir. Path",
	"Copy All Filenames", "Copy All File Paths",
}

func (state *model) copyDocumentPaths(command string) {
	doc := state.space.Current()
	value := ""
	switch command {
	case "Copy Current Full File path":
		value = doc.Path
		if value == "" {
			value = doc.Name
		}
	case "Copy Current Filename":
		value = doc.Name
	case "Copy Current Dir. Path":
		if doc.Path != "" {
			value = filepath.Dir(doc.Path)
		}
	case "Copy All Filenames", "Copy All File Paths":
		docs := state.space.Documents
		if state.splitEnabled {
			docs = append([]*document.Document(nil), state.paneTabs(0)...)
			docs = append(docs, state.paneTabs(1)...)
		}
		seen := make(map[*document.Document]bool, len(docs))
		var output strings.Builder
		for _, entry := range docs {
			if seen[entry] {
				continue
			}
			seen[entry] = true
			name := entry.Name
			if command == "Copy All File Paths" && entry.Path != "" {
				name = entry.Path
			}
			output.WriteString(name)
			output.WriteString("\r\n")
		}
		value = output.String()
	default:
		return
	}
	if err := app.SetClipboardText(value); err != nil {
		state.notice = "Copy failed: " + err.Error()
		return
	}
	state.notice = command
}

func (state *model) paintCopyPathsMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(320), size.Width-parentRight-4)
	height := float32(len(copyPathsCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+13*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range copyPathsCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := copyPathsScope.At(index)
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
		state.command(copyPathsCommands[selected])
	}
}
