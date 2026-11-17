package main

import (
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"vivapad/internal/document"
)

type documentSummary struct {
	path                                      string
	created, modified                         string
	characters, words, lines, bytes           int
	selectedCharacters, selectedBytes, ranges int
}

func summaryFor(doc *document.Document) documentSummary {
	value := doc.Editor.String()
	selection := doc.Editor.Selection()
	length, selectedBytes := doc.PhysicalUTF8Bytes(selection.Start, selection.End)
	summary := documentSummary{
		path:               doc.Path,
		characters:         utf8.RuneCountInString(value) - strings.Count(value, "\n") - strings.Count(value, "\r"),
		lines:              doc.LineCount(),
		bytes:              length,
		selectedCharacters: selection.End - selection.Start,
		selectedBytes:      selectedBytes,
		ranges:             1,
	}
	insideWord := false
	for _, character := range value {
		wordCharacter := unicode.IsLetter(character) || unicode.IsNumber(character) || character == '_'
		if wordCharacter && !insideWord {
			summary.words++
		}
		insideWord = wordCharacter
	}
	if doc.Path != "" {
		if info, err := os.Stat(doc.Path); err == nil {
			summary.modified = info.ModTime().Local().Format("2006-01-02 15:04:05")
			if created, ok := creationTime(info); ok {
				summary.created = created.Local().Format("2006-01-02 15:04:05")
			}
		}
	}
	return summary
}

func (state *model) paintSummary(canvas *gfx.Canvas, size gfx.Size) {
	canvas.SetEscapeTarget(summaryCloseID)
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Summary dialog", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width := min(float32(580), size.Width-24)
	height := min(float32(340), size.Height-24)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	smallLabel(canvas, "Summary", gfx.Rect{X: x + 18, Y: y + 12, Width: width - 36, Height: 26}, palette.text, 15)
	canvas.Fill(gfx.Rect{X: x + 18, Y: y + 43, Width: width - 36, Height: 1}, palette.border)
	printer := message.NewPrinter(language.English)
	rows := make([]string, 0, 8)
	if state.summary.path != "" {
		created, modified := state.summary.created, state.summary.modified
		if created == "" {
			created = "Unavailable"
		}
		if modified == "" {
			modified = "Unavailable"
		}
		rows = append(rows, "Full file path: "+state.summary.path, "Created: "+created, "Modified: "+modified)
	}
	rows = append(rows,
		"Characters (without line endings): "+printer.Sprint(state.summary.characters),
		"Words: "+printer.Sprint(state.summary.words),
		"Lines: "+printer.Sprint(state.summary.lines),
		"Document length: "+printer.Sprint(state.summary.bytes),
		printer.Sprintf("%d selected characters (%d bytes) in %d ranges", state.summary.selectedCharacters, state.summary.selectedBytes, state.summary.ranges),
	)
	rowHeight := min(float32(27), (height-100)/float32(len(rows)))
	for index, row := range rows {
		bounds := gfx.Rect{X: x + 18, Y: y + 52 + float32(index)*rowHeight, Width: width - 36, Height: rowHeight}
		smallLabel(canvas, canvas.FitLine(row, 11, gfx.WeightRegular, bounds.Width), bounds, palette.text, 11)
		canvas.HitWithOptions(summaryRowScope.At(index), bounds, row, gfx.HitOptions{Role: gfx.RoleGroup, Hint: row})
	}
	closeBounds := gfx.Rect{X: x + width - 100, Y: y + height - 43, Width: 80, Height: 28}
	canvas.Border(closeBounds, 1, palette.border, palette.toolbar)
	canvas.LabelCenter("OK", closeBounds, gfx.TextStyle{Size: 12, Color: palette.text})
	if hitButton(canvas, summaryCloseID, closeBounds, "Close Summary") {
		state.summaryOpen = false
	}
	canvas.PopLayer()
	canvas.PopLayer()
}
