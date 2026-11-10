package main

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func parseMarkExtended(value string) (string, error) {
	var expanded strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' {
			expanded.WriteByte(value[index])
			continue
		}
		index++
		if index >= len(value) {
			return "", fmt.Errorf("incomplete extended escape")
		}
		switch value[index] {
		case 'n':
			expanded.WriteByte('\n')
		case 'r':
			expanded.WriteByte('\r')
		case 't':
			expanded.WriteByte('\t')
		case '0':
			expanded.WriteByte(0)
		case '\\':
			expanded.WriteByte('\\')
		case 'x':
			if index+2 >= len(value) {
				return "", fmt.Errorf("incomplete hex escape")
			}
			decoded, err := strconv.ParseUint(value[index+1:index+3], 16, 8)
			if err != nil || decoded >= 0x80 {
				return "", fmt.Errorf("unsupported hex escape %q", value[index-1:index+3])
			}
			expanded.WriteByte(byte(decoded))
			index += 2
		default:
			return "", fmt.Errorf("unsupported extended escape \\%c", value[index])
		}
	}
	return expanded.String(), nil
}

func mergeMarkSpans(spans []text.Span) []text.Span {
	slices.SortFunc(spans, func(left, right text.Span) int { return left.Start - right.Start })
	merged := spans[:0]
	for _, span := range spans {
		if len(merged) > 0 && span.Start <= merged[len(merged)-1].End {
			merged[len(merged)-1].End = max(merged[len(merged)-1].End, span.End)
			continue
		}
		merged = append(merged, span)
	}
	return merged
}

func marksOutside(spans []text.Span, start, end int) []text.Span {
	var kept []text.Span
	for _, span := range spans {
		if span.End <= start || span.Start >= end {
			kept = append(kept, span)
			continue
		}
		if span.Start < start {
			kept = append(kept, text.Span{Start: span.Start, End: start})
		}
		if span.End > end {
			kept = append(kept, text.Span{Start: end, End: span.End})
		}
	}
	return kept
}

func (state *model) markSelectionRange(doc *document.Document, value string) (int, int, bool) {
	selection := doc.Editor.Selection()
	multi := state.multiSelections[state.activePane]
	rectangle := state.rectangles[state.activePane]
	if selection.Start >= selection.End || multi.active && multi.doc == doc && multi.value == value && len(multi.ranges) > 1 ||
		rectangle.active && rectangle.doc == doc && rectangle.value == value {
		state.notice = "Choose one ordinary selection to mark"
		return 0, 0, false
	}
	return doc.Editor.ByteOffset(selection.Start), doc.Editor.ByteOffset(selection.End), true
}

func (state *model) markCurrentDocument() {
	query := state.query.String()
	if query == "" {
		state.notice = "Enter text to mark"
		return
	}
	doc := state.space.Current()
	if state.markDialogOpen && state.markBookmarkLine {
		state.bookmarkLines(doc)
		if !state.bookmarksReady(doc) {
			return
		}
	}
	value := doc.Editor.String()
	options := state.searchOptions
	if state.markDialogOpen {
		switch state.markMode {
		case "extended":
			var err error
			query, err = parseMarkExtended(query)
			if err != nil {
				state.notice = err.Error()
				return
			}
			options.Regex = false
		case "regex":
			options.Regex, options.WholeWord = true, false
			if state.markDotAll {
				query = "(?s)" + query
			}
		default:
			options.Regex = false
		}
	}
	selectionStart, selectionEnd := 0, len(value)
	if state.markDialogOpen && state.markInSelection {
		var valid bool
		selectionStart, selectionEnd, valid = state.markSelectionRange(doc, value)
		if !valid {
			return
		}
	}
	spans, err := doc.MatchesWith(query, options)
	if err != nil {
		state.notice = err.Error()
		return
	}
	if state.markDialogOpen {
		if state.markInSelection {
			spans = slices.DeleteFunc(spans, func(span text.Span) bool { return span.Start < selectionStart || span.End > selectionEnd })
		} else if !state.markWrap {
			start := doc.Editor.ByteOffset(doc.Editor.Caret())
			spans = slices.DeleteFunc(spans, func(span text.Span) bool { return span.Start < start })
		}
	}
	stored := []text.Span(nil)
	state.syncFindMarks(doc)
	if state.markedDocument == doc && state.markedValue == value {
		stored = append(stored, state.markedSpans...)
	}
	if state.markDialogOpen && state.markPurge {
		if state.markInSelection {
			stored = marksOutside(stored, selectionStart, selectionEnd)
		} else {
			stored = nil
		}
	} else if !state.markDialogOpen {
		stored = nil
	}
	stored = mergeMarkSpans(append(stored, spans...))
	if state.markDialogOpen && state.markBookmarkLine {
		lines := append([]int(nil), state.bookmarkLines(doc)...)
		if state.markPurge {
			if state.markInSelection {
				from, to := strings.Count(value[:selectionStart], "\n"), strings.Count(value[:selectionEnd-1], "\n")
				lines = slices.DeleteFunc(lines, func(line int) bool { return line >= from && line <= to })
			} else {
				lines = nil
			}
		}
		for _, span := range spans {
			from, to := strings.Count(value[:span.Start], "\n"), strings.Count(value[:span.End-1], "\n")
			for line := from; line <= to; line++ {
				lines = append(lines, line)
			}
		}
		sort.Ints(lines)
		lines = slices.Compact(lines)
		state.setBookmarkLines(doc, lines)
	}
	state.markedDocument, state.markedValue, state.markedSpans = doc, value, stored
	state.markedTracking = trackIndicators(doc.Editor)
	state.notice = strconv.Itoa(len(spans)) + " occurrence(s) marked"
}

func (state *model) clearMarkDialog() {
	doc := state.space.Current()
	if state.markBookmarkLine && state.markInSelection {
		state.bookmarkLines(doc)
		if !state.bookmarksReady(doc) {
			return
		}
	}
	state.syncFindMarks(doc)
	value := doc.Editor.String()
	start, end := 0, len(value)
	if state.markInSelection {
		var valid bool
		start, end, valid = state.markSelectionRange(doc, value)
		if !valid {
			return
		}
	}
	if state.markedDocument == doc {
		if state.markedValue == value && state.markInSelection {
			state.markedSpans = marksOutside(state.markedSpans, start, end)
		} else {
			state.markedSpans = nil
		}
		if len(state.markedSpans) == 0 {
			state.markedDocument, state.markedValue = nil, ""
			state.markedTracking = indicatorTracker{}
		}
	}
	if state.markBookmarkLine {
		if state.markInSelection {
			from, to := strings.Count(value[:start], "\n"), strings.Count(value[:end-1], "\n")
			lines := slices.DeleteFunc(append([]int(nil), state.bookmarkLines(doc)...), func(line int) bool { return line >= from && line <= to })
			if len(lines) > 0 {
				state.setBookmarkLines(doc, lines)
			} else {
				delete(state.bookmarks, doc)
			}
		} else {
			delete(state.bookmarks, doc)
		}
	}
	state.notice = "Marks cleared"
}

func (state *model) copyMarkedText() {
	doc := state.space.Current()
	if !state.syncFindMarks(doc) {
		return
	}
	value := doc.Editor.String()
	if state.markedDocument != doc || state.markedValue != value || len(state.markedSpans) == 0 {
		state.notice = "No marks to copy"
		return
	}
	result, err := joinedMarkedText(value, state.markedSpans)
	if err != nil {
		state.notice = err.Error()
		return
	}
	if err := app.SetClipboardText(result); err != nil {
		state.notice = err.Error()
		return
	}
	state.notice = "Marked text copied"
}

func joinedMarkedText(value string, spans []text.Span) (string, error) {
	parts := make([]string, 0, len(spans))
	multiline := false
	for _, span := range spans {
		if span.Start < 0 || span.End > len(value) || span.Start >= span.End {
			return "", fmt.Errorf("marks are no longer valid")
		}
		part := value[span.Start:span.End]
		multiline = multiline || strings.Contains(part, "\n")
		parts = append(parts, part)
	}
	separator := "\r\n"
	if multiline {
		separator = "\r\n----\r\n"
	}
	result := strings.Join(parts, separator)
	if len(parts) > 1 {
		result += "\r\n"
	}
	return result, nil
}
