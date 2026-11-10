package document

import (
	"strings"

	"github.com/vivaui/viva/text"
)

func initialEmptyLineTargets(value string, start, end int, includeWhitespace bool) []ReplacementEdit {
	var changes []ReplacementEdit
	for position := start; position < end; {
		if position == 0 || value[position-1] == '\n' {
			matched, scan := position, position
			for scan < end {
				if includeWhitespace {
					for scan < end && (value[scan] == ' ' || value[scan] == '\t') {
						scan++
					}
				}
				newline := scan
				for scan < end && value[scan] == '\n' {
					scan++
				}
				if scan == newline {
					break
				}
				matched = scan
				if !includeWhitespace {
					break
				}
			}
			if matched > position {
				changes = append(changes, ReplacementEdit{Start: position, End: matched})
				position = matched
				continue
			}
		}
		newline := strings.IndexByte(value[position:end], '\n')
		if newline < 0 {
			break
		}
		position += newline + 1
	}
	return changes
}

func terminalEmptyLineTarget(value string, start int, includeWhitespace bool) []ReplacementEdit {
	lastNewline := strings.LastIndexByte(value, '\n')
	tailStart := lastNewline + 1
	tail := value[tailStart:]
	if tail != "" && (!includeWhitespace || strings.Trim(tail, " \t") != "") {
		return nil
	}
	if lastNewline >= start {
		first := lastNewline
		for first > start && value[first-1] == '\n' {
			first--
		}
		return []ReplacementEdit{{Start: first, End: len(value)}}
	}
	if includeWhitespace && tail != "" && tailStart >= start {
		return []ReplacementEdit{{Start: tailStart, End: len(value)}}
	}
	return nil
}

func (doc *Document) RemoveEmptyLines(includeWhitespace bool) int {
	removed := 0
	err := doc.Editor.Transact(func(editor *text.Editor) error {
		draft := &Document{Name: doc.Name, Editor: editor, newline: doc.newline, encoding: doc.encoding}
		value := editor.String()
		selection := editor.Selection()
		entire := selection.Start == selection.End
		start, end := 0, len(value)
		if !entire {
			start, end = editor.ByteOffset(selection.Start), editor.ByteOffset(selection.End)
		}
		changes := initialEmptyLineTargets(value, start, end, includeWhitespace)
		if len(changes) != 0 {
			batch, err := draft.PrepareTargetEdits(changes)
			if err != nil {
				return err
			}
			if !entire {
				batch.mapSelection(selection)
			}
			if err := batch.Apply(); err != nil {
				return err
			}
			for _, change := range changes {
				removed += strings.Count(value[change.Start:change.End], "\n")
			}
		}
		value = editor.String()
		start = 0
		if !entire {
			selection = editor.Selection()
			if editor.ByteOffset(selection.End) != len(value) {
				return nil
			}
			start = editor.ByteOffset(selection.Start)
		} else if newline := strings.LastIndexByte(value, '\n'); newline >= 0 {
			start = strings.LastIndexByte(value[:newline], '\n') + 1
		}
		changes = terminalEmptyLineTarget(value, start, includeWhitespace)
		if len(changes) == 0 {
			return nil
		}
		if err := draft.applyTargetEdits(changes); err != nil {
			return err
		}
		for _, change := range changes {
			removed += strings.Count(value[change.Start:change.End], "\n")
		}
		if removed == 0 {
			removed = 1
		}
		return nil
	})
	if err != nil {
		return 0
	}
	return removed
}

func (doc *Document) RemoveConsecutiveDuplicateLines() int {
	block := doc.sourceOrderBlock()
	if len(block.lines) < 2 {
		return 0
	}
	var changes []ReplacementEdit
	position, firstEnd, previousEnd, runCount := block.start, 0, 0, 0
	previous := ""
	for index, line := range block.lines {
		end := position + len(line)
		if index < len(block.lines)-1 || block.terminated {
			end++
		}
		full := doc.Editor.String()[position:end]
		if runCount > 0 && full == previous {
			runCount++
		} else {
			if runCount > 1 {
				changes = append(changes, ReplacementEdit{Start: firstEnd, End: previousEnd})
			}
			previous, firstEnd, runCount = full, end, 1
		}
		position, previousEnd = end, end
	}
	if runCount > 1 {
		changes = append(changes, ReplacementEdit{Start: firstEnd, End: previousEnd})
	}
	removed := 0
	reachesEOF := block.end == len(doc.Editor.String()) && !block.terminated
	err := doc.Editor.Transact(func(editor *text.Editor) error {
		draft := &Document{Name: doc.Name, Editor: editor, newline: doc.newline, encoding: doc.encoding}
		before := editor.String()
		if err := draft.applyTargetEdits(changes); err != nil {
			return err
		}
		for _, change := range changes {
			removed += strings.Count(before[change.Start:change.End], "\n")
		}
		value := editor.String()
		if !reachesEOF || strings.HasSuffix(value, "\n") {
			return nil
		}
		lastStart := strings.LastIndexByte(value, '\n') + 1
		if lastStart == 0 || lastStart == len(value) {
			return nil
		}
		previousStart := strings.LastIndexByte(value[:lastStart-1], '\n') + 1
		if value[previousStart:lastStart-1] != value[lastStart:] {
			return nil
		}
		if err := draft.applyTargetEdits([]ReplacementEdit{{Start: previousStart, End: lastStart}}); err != nil {
			return err
		}
		removed++
		return nil
	})
	if err != nil {
		return 0
	}
	return removed
}
