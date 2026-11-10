package main

import (
	"sort"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/linemarks"
)

type bookmarkState struct {
	value      string
	lines      []int
	editor     *text.Editor
	cursor     uint64
	incomplete bool
}

func (state *model) setBookmarkLines(doc *document.Document, lines []int) {
	state.bookmarks[doc] = &bookmarkState{
		value: doc.Editor.String(), lines: append([]int(nil), lines...),
		editor: doc.Editor, cursor: doc.Editor.TrackLineEdits(),
	}
}

func bookmarkLine(doc *document.Document) int {
	value := doc.Editor.String()
	return strings.Count(value[:doc.Editor.ByteOffset(doc.Editor.Caret())], "\n")
}

func (state *model) bookmarkLines(doc *document.Document) []int {
	stored := state.bookmarks[doc]
	if stored == nil {
		return nil
	}
	if stored.incomplete {
		state.bookmarksReady(doc)
		return nil
	}
	value := doc.Editor.String()
	edits, next, complete := doc.Editor.LineEditsSince(stored.cursor, nil)
	if complete && stored.editor == doc.Editor && len(edits) == 0 && stored.value == value {
		return stored.lines
	}
	events := make([]linemarks.LineEdit, 0, len(edits))
	for _, edit := range edits {
		events = append(events, linemarks.LineEdit{
			Start: edit.Start, Removed: edit.Removed, Inserted: edit.Inserted,
			Line: edit.Line, RemovedLines: edit.RemovedLines, InsertedLines: edit.InsertedLines, AtLineStart: edit.AtLineStart,
		})
	}
	mapped, valid := linemarks.MapLines(stored.value, value, stored.lines, events, complete && stored.editor == doc.Editor)
	if !valid {
		stored.incomplete = true
		state.bookmarksReady(doc)
		return nil
	}
	stored.value, stored.lines, stored.cursor = value, mapped, next
	return stored.lines
}

func (state *model) bookmarksReady(doc *document.Document) bool {
	if stored := state.bookmarks[doc]; stored != nil && stored.incomplete {
		state.notice = "Bookmark history unavailable; clear bookmarks or reload"
		return false
	}
	return true
}

func (state *model) editWithBookmarkAnchor(doc *document.Document, edit func()) {
	state.bookmarkLines(doc)
	edit()
	state.bookmarkLines(doc)
}

func (state *model) toggleBookmark() {
	doc := state.space.Current()
	lines := state.bookmarkLines(doc)
	if !state.bookmarksReady(doc) {
		return
	}
	stored := state.bookmarks[doc]
	if stored == nil {
		state.setBookmarkLines(doc, nil)
		stored = state.bookmarks[doc]
	}
	line := bookmarkLine(doc)
	index := sort.SearchInts(lines, line)
	if index < len(lines) && lines[index] == line {
		stored.lines = append(lines[:index], lines[index+1:]...)
	} else {
		stored.lines = append(lines, 0)
		copy(stored.lines[index+1:], stored.lines[index:])
		stored.lines[index] = line
	}
}

func (state *model) navigateBookmark(previous bool) {
	doc := state.space.Current()
	lines := state.bookmarkLines(doc)
	if !state.bookmarksReady(doc) || len(lines) == 0 {
		return
	}
	current := bookmarkLine(doc)
	index := sort.SearchInts(lines, current)
	if previous {
		if index == 0 {
			index = len(lines)
		}
		index--
	} else {
		if index < len(lines) && lines[index] == current {
			index++
		}
		if index == len(lines) {
			index = 0
		}
	}
	doc.GoToLine(lines[index] + 1)
}

func (state *model) inverseBookmarks() {
	doc := state.space.Current()
	marked := state.bookmarkLines(doc)
	if !state.bookmarksReady(doc) {
		return
	}
	count := doc.LineCount()
	lines := make([]int, 0, count-len(marked))
	for line, index := 0, 0; line < count; line++ {
		if index < len(marked) && marked[index] == line {
			index++
		} else {
			lines = append(lines, line)
		}
	}
	state.setBookmarkLines(doc, lines)
}

func (state *model) copyBookmarkedLines(cut bool) {
	doc := state.space.Current()
	marked := state.bookmarkLines(doc)
	if !state.bookmarksReady(doc) {
		return
	}
	value := doc.Editor.String()
	lines := strings.Split(value, "\n")
	var copied strings.Builder
	for _, line := range marked {
		copied.WriteString(lines[line])
		if line < len(lines)-1 {
			copied.WriteString(doc.LineEndingAt(line))
		}
	}
	clipboard := copied.String()
	if err := app.SetClipboardText(clipboard); err != nil {
		state.notice = err.Error()
		return
	}
	if cut {
		state.removeBookmarkedLines(true)
	}
}

func (state *model) removeBookmarkedLines(removeMarked bool) {
	doc := state.space.Current()
	marked := state.bookmarkLines(doc)
	if !state.bookmarksReady(doc) {
		return
	}
	value := doc.Editor.String()
	lines := strings.Split(value, "\n")
	var edits []text.Replacement
	var keptMarks []int
	position, nextMark := 0, 0
	for line, content := range lines {
		isMarked := nextMark < len(marked) && marked[nextMark] == line
		if isMarked {
			nextMark++
		}
		end := position + len(content)
		if line < len(lines)-1 {
			end++
		}
		if isMarked == removeMarked {
			edits = append(edits, text.Replacement{Range: text.Range{Start: doc.Editor.RuneIndex(position), End: doc.Editor.RuneIndex(end)}})
		} else if isMarked {
			keptMarks = append(keptMarks, line)
		}
		position = end
	}
	state.setBookmarkLines(doc, keptMarks)
	state.editWithBookmarkAnchor(doc, func() {
		if err := doc.Editor.ReplaceRanges(edits); err != nil {
			state.setBookmarkLines(doc, marked)
			state.notice = err.Error()
		}
	})
}

func (state *model) pasteToBookmarkedLines() {
	doc := state.space.Current()
	marked := state.bookmarkLines(doc)
	if !state.bookmarksReady(doc) || len(marked) == 0 {
		return
	}
	clipboard, err := app.ClipboardText()
	if err != nil {
		state.notice = err.Error()
		return
	}
	clipboard = strings.ReplaceAll(strings.ReplaceAll(clipboard, "\r\n", "\n"), "\r", "\n")
	value := doc.Editor.String()
	lines := strings.Split(value, "\n")
	var edits []text.Replacement
	position, nextMark := 0, 0
	for line, content := range lines {
		end := position + len(content)
		if nextMark < len(marked) && marked[nextMark] == line {
			edits = append(edits, text.Replacement{Range: text.Range{Start: doc.Editor.RuneIndex(position), End: doc.Editor.RuneIndex(end)}, Text: clipboard})
			nextMark++
		}
		position = end + 1
	}
	state.editWithBookmarkAnchor(doc, func() {
		if err := doc.Editor.ReplaceRanges(edits); err != nil {
			state.notice = err.Error()
		}
	})
}

func (state *model) deleteLineWithBookmarks(doc *document.Document) {
	state.editWithBookmarkAnchor(doc, doc.DeleteLine)
}

func (state *model) applyCodeCommand(doc *document.Document, command components.CodeEditorCommand) {
	switch command {
	case components.CodeEditorDeleteLines:
		state.deleteLineWithBookmarks(doc)
	case components.CodeEditorDuplicateLinesUp, components.CodeEditorDuplicateLinesDown,
		components.CodeEditorMoveLinesUp, components.CodeEditorMoveLinesDown:
		state.rearrangeBookmarkedLines(doc, command)
	default:
		state.editWithBookmarkAnchor(doc, func() { command.Apply(doc.Editor) })
	}
}

func (state *model) rearrangeBookmarkedLines(doc *document.Document, command components.CodeEditorCommand) {
	if command == components.CodeEditorMoveLinesUp || command == components.CodeEditorMoveLinesDown {
		state.moveLinesWithBookmarks(doc, command == components.CodeEditorMoveLinesUp)
		return
	}
	state.editWithBookmarkAnchor(doc, func() { command.Apply(doc.Editor) })
}

func (state *model) rebindBookmarksOnReload(previous, current *document.Document) {
	marked := state.bookmarkLines(previous)
	if len(marked) == 0 {
		return
	}
	before, after := previous.Editor.String(), current.Editor.String()
	if before == after {
		state.setBookmarkLines(current, marked)
		return
	}
	oldLines, newLines := strings.Split(before, "\n"), strings.Split(after, "\n")
	oldCounts, newCounts := make(map[string]int), make(map[string]int)
	newPositions := make(map[string]int)
	for _, line := range oldLines {
		oldCounts[line]++
	}
	for index, line := range newLines {
		newCounts[line]++
		newPositions[line] = index
	}
	prefix := 0
	for prefix < min(len(oldLines), len(newLines)) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix &&
		oldLines[len(oldLines)-suffix-1] == newLines[len(newLines)-suffix-1] {
		suffix++
	}
	mapped := make([]int, 0, len(marked))
	for _, index := range marked {
		line := oldLines[index]
		switch {
		case oldCounts[line] == 1 && newCounts[line] == 1:
			mapped = append(mapped, newPositions[line])
		case oldCounts[line] == newCounts[line] && newCounts[line] > 0 && index < prefix:
			mapped = append(mapped, index)
		case oldCounts[line] == newCounts[line] && newCounts[line] > 0 && index >= len(oldLines)-suffix:
			mapped = append(mapped, index+len(newLines)-len(oldLines))
		}
	}
	if len(mapped) != 0 {
		sort.Ints(mapped)
		state.setBookmarkLines(current, mapped)
	}
}
