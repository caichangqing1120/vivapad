package main

import (
	"runtime"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

var readOnlyCommands = [...]string{
	"Read-Only on Current Document", "Read-Only for All Documents", "Clear Read-Only for All Documents",
}

func (state *model) isReadOnly(doc *document.Document) bool {
	if doc == nil {
		return false
	}
	if doc.LargeFile() != nil || state.readOnly[doc] || state.monitors[doc] != nil {
		return true
	}
	readonly, _ := document.FileReadOnly(doc.Path)
	return readonly
}

func fileReadOnlyLabel() string {
	if runtime.GOOS == "windows" {
		return "Read-Only Attribute in Windows"
	}
	return "File Read-Only Attribute"
}

func (state *model) restoreSessionReadOnly(flags map[string]bool) {
	for _, doc := range state.space.Documents {
		if flags[doc.Path] {
			state.readOnly[doc] = true
		}
	}
}

func (state *model) textMutation(command string) bool {
	if tool, _ := findTextTool(command); tool.transform != nil {
		return true
	}
	if _, ok := document.EncodingFromLabel(command); ok {
		return true
	}
	if _, _, ok := numericSortForCommand(command); ok {
		return true
	}
	if _, ok := blankOperationForCommand(command); ok {
		return true
	}
	switch command {
	case pasteHTMLCommand, pasteRTFCommand:
		return true
	case "UTF-8", "UTF-8 BOM", "UTF-16 LE", "UTF-16 BE":
		return true
	case "Undo", "Redo", "Cut", "Paste", "Delete", "Redact Selection", "Redact Selection with Bullets", "Word Completion", "Path Completion", "Column Editor...", "Duplicate Line", "Delete Line", "Indent Lines", "Unindent Lines",
		"Unix (LF)", "Windows (CR LF)", "Macintosh (CR)",
		"Toggle Single Line Comment", "Single Line Comment", "Single Line Uncomment", "Block Comment", "Block Uncomment",
		"Date Time (short)", "Date Time (long)", "Date Time (customized)",
		"Join Lines", "Reverse Line Order", "Insert Blank Line Above Current", "Insert Blank Line Below Current", "Split Lines", "Move Up Current Line", "Move Down Current Line",
		"Sort In Locale Order Ascending", "Sort In Locale Order Descending", "Randomize Line Order",
		"UPPERCASE", "lowercase", "Proper Case", "Proper Case (blend)", "Sentence case", "Sentence case (blend)", "Invert Case", "Random Case", "Sort Lines Ascending", "Sort Lines Descending", "Remove Duplicate Lines",
		"Sort Ascending (Ignore Case)", "Sort Descending (Ignore Case)", "Remove Empty Lines", "Remove Blank Lines",
		"Remove Consecutive Duplicates", "Playback Macro", "Playback 5 Times", "Cut Bookmarked Lines",
		"Paste to (Replace) Bookmarked Lines", "Remove Bookmarked Lines", "Remove Non-Bookmarked Lines":
		return true
	}
	return false
}

func (state *model) paintReadOnlyMenu(canvas *gfx.Canvas, size gfx.Size, parentRight float32) {
	palette := state.palette()
	width := min(float32(310), size.Width-parentRight-4)
	height := float32(len(readOnlyCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+16*27, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.popup)
	selected := -1
	for index, command := range readOnlyCommands {
		item := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := readOnlyScope.At(index)
		if canvas.Hovered() == id {
			canvas.Fill(item, palette.hover)
		}
		if index == 0 && state.readOnly[state.space.Current()] {
			canvas.DrawIcon(checkIcon, gfx.Rect{X: item.X + 5, Y: item.Y + 6, Width: 14, Height: 14}, palette.accent)
		}
		label := canvas.FitLine(command, 11, gfx.WeightRegular, item.Width-39)
		smallLabel(canvas, label, gfx.Rect{X: item.X + 26, Y: item.Y, Width: item.Width - 32, Height: item.Height}, palette.text, 11)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem, Checked: index == 0 && state.readOnly[state.space.Current()]})
		if canvas.Clicked(id) {
			selected = index
		}
	}
	canvas.PopLayer()
	if selected >= 0 {
		state.command(readOnlyCommands[selected])
	}
}
