package printing

import (
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

type ColorRun struct {
	StartByte, EndByte int
	Color              ui.Color
}

func Colorize(job Job, pages []Page, highlight func(string, string, []components.CodeEditorRun) []components.CodeEditorRun) ([]Page, error) {
	if !utf8.ValidString(job.Text) {
		return nil, fmt.Errorf("printing: source is not UTF-8")
	}
	colored := append([]Page(nil), pages...)
	if highlight == nil || job.Settings.ColorMode != ColorOnWhite {
		return colored, nil
	}
	lineStarts := []int{0}
	for index, character := range job.Text {
		if character == '\n' {
			lineStarts = append(lineStarts, index+1)
		}
	}
	cache := make(map[int][]components.CodeEditorRun)
	for pageIndex, page := range pages {
		colored[pageIndex].Styles = make([][]ColorRun, len(page.Rows))
		for rowIndex, row := range page.Rows {
			if row.StartByte < 0 || row.EndByte < row.StartByte || row.EndByte > len(job.Text) {
				return nil, fmt.Errorf("printing: invalid row source span")
			}
			lineIndex := sort.SearchInts(lineStarts, row.StartByte+1) - 1
			lineStart, lineEnd := lineStarts[lineIndex], len(job.Text)
			if lineIndex+1 < len(lineStarts) {
				lineEnd = lineStarts[lineIndex+1] - 1
			}
			if row.EndByte > lineEnd || !runeBoundary(job.Text, row.StartByte) || !runeBoundary(job.Text, row.EndByte) {
				return nil, fmt.Errorf("printing: row crosses a source line or rune")
			}
			runs, known := cache[lineStart]
			if !known {
				line := job.Text[lineStart:lineEnd]
				runs = highlight(job.Language, line, nil)
				last := 0
				for _, run := range runs {
					if run.Start < last || run.End <= run.Start || run.End > len(line) ||
						!runeBoundary(line, run.Start) || !runeBoundary(line, run.End) {
						return nil, fmt.Errorf("printing: invalid syntax run")
					}
					last = run.End
				}
				cache[lineStart] = runs
			}
			for _, run := range runs {
				start := max(row.StartByte, lineStart+run.Start)
				end := min(row.EndByte, lineStart+run.End)
				if end > start {
					colored[pageIndex].Styles[rowIndex] = append(colored[pageIndex].Styles[rowIndex], ColorRun{
						StartByte: start, EndByte: end, Color: run.Color,
					})
				}
			}
		}
	}
	return colored, nil
}

func runeBoundary(value string, offset int) bool {
	return offset == len(value) || offset >= 0 && offset < len(value) && utf8.RuneStart(value[offset])
}
