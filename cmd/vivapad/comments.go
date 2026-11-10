package main

import (
	"strings"

	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/macro"
)

var commentCommands = [...]string{
	"Toggle Single Line Comment", "Single Line Comment", "Single Line Uncomment",
	"Block Comment", "Block Uncomment",
}

func lineCommentForLanguage(language string) string {
	switch strings.ToLower(language) {
	case ".go", ".c", ".cpp", ".h", ".js", ".ts", ".java", ".rs":
		return "//"
	case ".py", ".sh", ".yaml", ".yml", ".toml":
		return "#"
	case ".sql":
		return "--"
	}
	return ""
}

func blockCommentForLanguage(language string) (string, string) {
	switch strings.ToLower(language) {
	case ".go", ".c", ".cpp", ".h", ".js", ".ts", ".java", ".rs", ".css", ".sql":
		return "/*", "*/"
	case ".html":
		return "<!--", "-->"
	}
	return "", ""
}

func commentStep(command, language string) (macro.Step, bool) {
	action := ""
	switch command {
	case "Toggle Single Line Comment":
		action = "line-toggle"
	case "Single Line Comment":
		action = "line-add"
	case "Single Line Uncomment":
		action = "line-remove"
	case "Block Comment":
		action = "block-add"
	case "Block Uncomment":
		action = "block-remove"
	}
	if action == "" {
		return macro.Step{}, false
	}
	start, end := blockCommentForLanguage(language)
	step := macro.Step{Kind: macro.CommentStep, Comment: action, LinePrefix: lineCommentForLanguage(language), BlockStart: start, BlockEnd: end}
	return step, step.LinePrefix != "" || start != ""
}

func (state *model) applyCommentCommand(doc *document.Document, command string) {
	step, ok := commentStep(command, state.languageFor(doc))
	if !ok {
		state.notice = "No comment for this language"
		return
	}
	changed := false
	state.editWithBookmarkAnchor(doc, func() { changed = macro.ApplyComment(doc, step) })
	if changed {
		state.notice = command
		state.recordMacro(doc, step)
	} else {
		state.notice = "No comment changes"
	}
}

func (state *model) paintCommentMenu(canvas *ui.Canvas, size ui.Size, parentRight float32) {
	width := min(float32(235), size.Width-parentRight-4)
	height := float32(len(commentCommands))*27 + 8
	x := parentRight
	y := min(menuHeight+4+18*18, max(menuHeight, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(ui.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.white)
	selected := -1
	for index, command := range commentCommands {
		item := ui.Rect{X: x + 4, Y: y + 4 + float32(index)*27, Width: width - 8, Height: 27}
		id := commentScope.At(index)
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
		state.command(commentCommands[selected])
	}
}
