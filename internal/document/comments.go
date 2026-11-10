package document

import (
	"slices"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
)

type LineCommentMode uint8

const (
	LineCommentAdd LineCommentMode = iota + 1
	LineCommentRemove
	LineCommentToggle
)

type commentChange struct {
	at, removed int
	inserted    string
	afterCaret  bool
}

func (doc *Document) ChangeLineComments(prefix string, mode LineCommentMode) bool {
	return doc.changeLineComments(prefix, "", mode)
}

func (doc *Document) ChangeStreamLineComments(startDelimiter, endDelimiter string, mode LineCommentMode) bool {
	if endDelimiter == "" {
		return false
	}
	return doc.changeLineComments(startDelimiter, endDelimiter, mode)
}

func (doc *Document) changeLineComments(prefix, suffix string, mode LineCommentMode) bool {
	if prefix == "" || mode < LineCommentAdd || mode > LineCommentToggle {
		return false
	}
	editor := doc.Editor
	value := []rune(editor.String())
	selection := editor.Selection()
	start := selection.Start
	for start > 0 && value[start-1] != '\n' {
		start--
	}
	last := selection.End
	if last > selection.Start && value[last-1] == '\n' {
		last--
	}
	end := last
	for end < len(value) && value[end] != '\n' {
		end++
	}
	mark := []rune(prefix)
	endMark := []rune(suffix)
	changes := make([]commentChange, 0, 4)
	for position := start; ; {
		lineEnd := position
		for lineEnd < end && value[lineEnd] != '\n' {
			lineEnd++
		}
		line := value[position:lineEnd]
		indent := 0
		for indent < len(line) && (line[indent] == ' ' || line[indent] == '\t') {
			indent++
		}
		if indent < len(line) {
			commented := commentPrefixAt(line, indent, mark) && (suffix == "" ||
				len(line)-len(endMark) >= indent+len(mark) && commentPrefixAt(line, len(line)-len(endMark), endMark))
			switch {
			case mode != LineCommentAdd && commented:
				openEnd := indent + len(mark)
				closeStart := len(line) - len(endMark)
				if openEnd < closeStart && line[openEnd] == ' ' {
					openEnd++
				}
				if suffix != "" && closeStart > openEnd && line[closeStart-1] == ' ' {
					closeStart--
				}
				changes = append(changes, commentChange{at: position + indent, removed: openEnd - indent})
				if suffix != "" {
					changes = append(changes, commentChange{at: position + closeStart, removed: len(line) - closeStart})
				}
			case mode != LineCommentRemove:
				changes = append(changes, commentChange{at: position + indent, inserted: prefix + " "})
				if suffix != "" {
					changes = append(changes, commentChange{at: lineEnd, inserted: " " + suffix, afterCaret: true})
				}
			}
		}
		if lineEnd == end {
			break
		}
		position = lineEnd + 1
	}
	if len(changes) == 0 {
		return false
	}
	newStart, newEnd := mapCommentPosition(selection.Start, changes), mapCommentPosition(selection.End, changes)
	return doc.applyCommentChanges(changes, text.Range{Start: newStart, End: newEnd})
}

func mapCommentPosition(position int, changes []commentChange) int {
	delta := 0
	for _, change := range changes {
		if position < change.at {
			break
		}
		if position == change.at && change.afterCaret {
			continue
		}
		if change.removed > 0 && position < change.at+change.removed {
			return change.at + delta
		}
		delta += utf8.RuneCountInString(change.inserted) - change.removed
	}
	return position + delta
}

func (doc *Document) AddBlockComment(startDelimiter, endDelimiter string) bool {
	if startDelimiter == "" || endDelimiter == "" {
		return false
	}
	editor := doc.Editor
	value := []rune(editor.String())
	selection := editor.Selection()
	start, end := selection.Start, selection.End
	if start == end {
		for start > 0 && value[start-1] != '\n' {
			start--
		}
		for start < len(value) && (value[start] == ' ' || value[start] == '\t') {
			start++
		}
		for end < len(value) && value[end] != '\n' {
			end++
		}
	}
	opening, closing := startDelimiter+" ", " "+endDelimiter
	inner := text.Range{Start: start + len([]rune(opening)), End: start + len([]rune(opening)) + end - start}
	changes := []commentChange{{at: start, inserted: opening}, {at: end, inserted: closing}}
	if start == end {
		changes = []commentChange{{at: start, inserted: opening + closing}}
	}
	return doc.applyCommentChanges(changes, inner)
}

type commentPair struct {
	start, end int
}

func (doc *Document) RemoveBlockComments(startDelimiter, endDelimiter string) bool {
	if startDelimiter == "" || endDelimiter == "" {
		return false
	}
	editor := doc.Editor
	value := []rune(editor.String())
	opening, closing := []rune(startDelimiter), []rune(endDelimiter)
	selection := editor.Selection()
	pairs := make([]commentPair, 0, 2)
	for position := 0; position < len(value); {
		if !commentPrefixAt(value, position, opening) {
			position++
			continue
		}
		end := position + len(opening)
		for end < len(value) && !commentPrefixAt(value, end, closing) {
			end++
		}
		if end == len(value) {
			break
		}
		last := end + len(closing)
		if position <= selection.Start && selection.Start < last ||
			position < selection.End && selection.End <= last ||
			selection.Start <= position && last <= selection.End {
			pairs = append(pairs, commentPair{start: position, end: end})
		}
		position = last
	}
	if len(pairs) == 0 {
		return false
	}
	changes := make([]commentChange, 0, 2*len(pairs))
	for _, pair := range pairs {
		openEnd := pair.start + len(opening)
		if openEnd < pair.end && value[openEnd] == ' ' {
			openEnd++
		}
		closeStart := pair.end
		if closeStart > openEnd && value[closeStart-1] == ' ' {
			closeStart--
		}
		changes = append(changes, commentChange{at: pair.start, removed: openEnd - pair.start},
			commentChange{at: closeStart, removed: pair.end + len(closing) - closeStart})
	}
	newStart, newEnd := mapCommentPosition(selection.Start, changes), mapCommentPosition(selection.End, changes)
	return doc.applyCommentChanges(changes, text.Range{Start: newStart, End: newEnd})
}

func (doc *Document) applyCommentChanges(changes []commentChange, after text.Range) bool {
	if len(changes) == 0 {
		return false
	}
	selection := doc.Editor.Selection()
	if selection.Start != selection.End && doc.Editor.Caret() == selection.Start {
		after = text.Range{Start: after.End, End: after.Start}
	}
	replacements := make([]text.Replacement, 0, len(changes))
	for _, change := range changes {
		replacements = append(replacements, text.Replacement{
			Range: text.Range{Start: change.at, End: change.at + change.removed}, Text: change.inserted,
		})
	}
	return doc.Editor.ReplaceRangesWith(replacements, text.ReplaceRangesOptions{Forward: true, AfterSelection: &after}) == nil
}

func commentPrefixAt(value []rune, position int, prefix []rune) bool {
	return position+len(prefix) <= len(value) && slices.Equal(value[position:position+len(prefix)], prefix)
}
