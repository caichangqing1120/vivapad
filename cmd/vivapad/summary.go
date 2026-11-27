package main

import (
	"os"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
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
	return summarizeSelections(doc, []text.Range{doc.Editor.Selection()}, false)
}

func (state *model) summaryForPane(doc *document.Document, pane int) documentSummary {
	value := doc.Editor.String()
	if rect := state.rectangles[pane]; rect.active && rect.doc == doc && rect.value == value {
		rows := doc.RectangleRows(state.rectangleForPane(pane))
		if len(rows) > 0 {
			ranges := make([]text.Range, 0, len(rows))
			for _, row := range rows {
				ranges = append(ranges, row.Range)
			}
			return summarizeSelections(doc, ranges, true)
		}
	}
	if multi := state.multiSelections[pane]; multi.active && multi.doc == doc && multi.value == value &&
		len(multi.ranges) > 0 && multi.ranges[len(multi.ranges)-1] == doc.Editor.Selection() {
		return summarizeSelections(doc, multi.ranges, false)
	}
	return summaryFor(doc)
}

func summarizeSelections(doc *document.Document, ranges []text.Range, rectangle bool) documentSummary {
	counts := doc.SummaryCountsForRanges(ranges)
	characters := 0
	for _, r := range ranges {
		characters += max(r.Start, r.End) - min(r.Start, r.End)
	}
	areas := len(ranges)
	if areas == 1 && characters == 0 {
		areas = 0
	} else if rectangle && areas > 1 {
		areas = 1
	}
	summary := documentSummary{
		path:               doc.Path,
		characters:         counts.Characters,
		words:              counts.Words,
		lines:              counts.Lines,
		bytes:              counts.Bytes,
		selectedCharacters: characters,
		selectedBytes:      counts.SelectedBytes,
		ranges:             areas,
	}
	if doc.Path != "" {
		if info, err := os.Stat(doc.Path); err == nil {
			summary.modified = info.ModTime().Local().Format("2006-01-02 15:04:05")
			if created, ok := creationTime(doc.Path, info); ok {
				summary.created = created.Local().Format("2006-01-02 15:04:05")
			}
		}
	}
	return summary
}

func (state *model) paintSummary(canvas *gfx.Canvas, size gfx.Size) {
	palette := state.palette()
	canvas.SetEscapeTarget(summaryCloseID)
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Summary dialog", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width := min(float32(580), size.Width-24)
	height := min(float32(340), size.Height-24)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.popup)
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
		canvas.RequestFocus(state.commandEditorFocusID())
	}
	canvas.PopLayer()
	canvas.PopLayer()
}
