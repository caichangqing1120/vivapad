package main

import (
	"strings"
	"unicode/utf8"
)

func rebaseRectangleOnReload(selection rectangleSelection, before, after string) (rectangleSelection, bool) {
	if selection.value != before || !selection.active && !selection.pending {
		return rectangleSelection{}, false
	}
	if before == after {
		selection.copyText, selection.copyValid = "", false
		return selection, true
	}
	if selection.pending {
		return rectangleSelection{}, false
	}
	oldLines, newLines := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(oldLines) != len(newLines) {
		return rectangleSelection{}, false
	}
	anchorLine, anchorColumn, okAnchor := rectangleReloadPosition(oldLines, selection.anchor)
	caretLine, caretColumn, okCaret := rectangleReloadPosition(oldLines, selection.caret)
	if !okAnchor || !okCaret {
		return rectangleSelection{}, false
	}
	for line := min(anchorLine, caretLine); line <= max(anchorLine, caretLine); line++ {
		if oldLines[line] != newLines[line] {
			return rectangleSelection{}, false
		}
	}
	selection.anchor = rectangleReloadIndex(newLines, anchorLine, anchorColumn)
	selection.caret = rectangleReloadIndex(newLines, caretLine, caretColumn)
	selection.value, selection.copyText, selection.copyValid = after, "", false
	return selection, true
}

func rectangleReloadPosition(lines []string, index int) (line, column int, ok bool) {
	if index < 0 {
		return 0, 0, false
	}
	for line, value := range lines {
		width := utf8.RuneCountInString(value)
		if index <= width {
			return line, index, true
		}
		index -= width + 1
	}
	return 0, 0, false
}

func rectangleReloadIndex(lines []string, line, column int) int {
	index := column
	for previous := 0; previous < line; previous++ {
		index += utf8.RuneCountInString(lines[previous]) + 1
	}
	return index
}
