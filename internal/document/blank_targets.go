package document

import (
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
)

func (doc *Document) ApplyBlankOperation(operation BlankOperation) bool {
	if doc == nil || doc.Editor == nil || operation < BlankTrimTrailing || operation > BlankSpaceToTabLeading {
		return false
	}
	if operation != BlankTrimAndJoin {
		changed, err := doc.applyBlankPhase(operation)
		return changed && err == nil
	}
	changed := false
	err := doc.Editor.Transact(func(editor *text.Editor) error {
		draft := &Document{Editor: editor}
		selection := editor.Selection()
		entire := selection.Start == selection.End
		trimmed, err := draft.applyBlankPhase(BlankTrimBoth)
		if err != nil {
			return err
		}
		changed = trimmed
		selection = editor.Selection()
		if entire || selection.Start != selection.End {
			joined, err := draft.applyBlankPhase(BlankEOLToSpace)
			changed = changed || joined
			return err
		}
		return nil
	})
	return changed && err == nil
}

func (doc *Document) applyBlankPhase(operation BlankOperation) (bool, error) {
	editor := doc.Editor
	selection := editor.Selection()
	after := selection
	if editor.Caret() == selection.Start {
		after = text.Range{Start: selection.End, End: selection.Start}
	}
	value := editor.String()
	var changes []ReplacementEdit
	if operation == BlankEOLToSpace {
		start, end := 0, len(value)
		if selection.Start != selection.End {
			start, end = editor.ByteOffset(selection.Start), editor.ByteOffset(selection.End)
		}
		changes = blankJoinTargets(value, start, end)
	} else {
		block := doc.selectedLineBlock()
		changes = doc.blankLineTargets(block, operation)
		if selection.Start != selection.End {
			end := block.end
			if block.terminated && editor.ByteOffset(selection.End) != end {
				end--
			}
			after = text.Range{Start: editor.RuneIndex(block.start), End: editor.RuneIndex(end)}
		}
	}
	if len(changes) == 0 {
		return false, nil
	}
	replacements := make([]text.Replacement, 0, len(changes))
	shift := 0
	for _, change := range changes {
		start, end := editor.RuneIndex(change.Start), editor.RuneIndex(change.End)
		inserted := utf8.RuneCountInString(change.Text)
		after = moveTargetSelection(after, start+shift, end+shift, inserted)
		shift += inserted - (end - start)
		replacements = append(replacements, text.Replacement{Range: text.Range{Start: start, End: end}, Text: change.Text})
	}
	if operation >= BlankTabToSpace && selection.Start == selection.End {
		position := doc.blankTabCaret(value, changes, operation)
		after = text.Range{Start: position, End: position}
	}
	err := editor.ReplaceRangesWith(replacements, text.ReplaceRangesOptions{Forward: true, AfterSelection: &after})
	return err == nil, err
}

func (doc *Document) blankLineTargets(block lineBlock, operation BlankOperation) []ReplacementEdit {
	var changes []ReplacementEdit
	position := block.start
	for _, line := range block.lines {
		if operation >= BlankTabToSpace {
			converted, _ := convertBlankLine(line, doc.blankTabWidth(), operation, -1)
			if converted != line {
				changes = append(changes, ReplacementEdit{Start: position, End: position + len(line), Text: converted})
			}
		} else {
			head := len(line) - len(strings.TrimLeft(line, " \t"))
			tail := len(strings.TrimRight(line, " \t"))
			if operation != BlankTrimTrailing && head > 0 {
				changes = append(changes, ReplacementEdit{Start: position, End: position + head})
			}
			if operation != BlankTrimLeading && tail < len(line) && (operation == BlankTrimTrailing || head < len(line)) {
				changes = append(changes, ReplacementEdit{Start: position + tail, End: position + len(line)})
			}
		}
		position += len(line) + 1
	}
	return changes
}

func blankJoinTargets(value string, start, end int) []ReplacementEdit {
	var changes []ReplacementEdit
	previous := byte(0)
	if start > 0 {
		previous = value[start-1]
	}
	for position := start; position < end; position++ {
		if value[position] != '\n' {
			previous = value[position]
			continue
		}
		replacement := ""
		if previous != ' ' {
			replacement = " "
			previous = ' '
		}
		changes = append(changes, ReplacementEdit{Start: position, End: position + 1, Text: replacement})
	}
	return changes
}

func (doc *Document) blankTabWidth() int {
	width := doc.lineOptions().TabWidth
	if width <= 0 {
		return 4
	}
	return width
}

func (doc *Document) blankTabCaret(value string, changes []ReplacementEdit, operation BlankOperation) int {
	editor := doc.Editor
	caret := editor.ByteOffset(editor.Caret())
	start := strings.LastIndexByte(value[:caret], '\n') + 1
	end := len(value)
	if newline := strings.IndexByte(value[start:], '\n'); newline >= 0 {
		end = start + newline
	}
	converted, position := convertBlankLine(value[start:end], doc.blankTabWidth(), operation, caret-start)
	point := editor.RuneIndex(start) + utf8.RuneCountInString(converted[:min(position, len(converted))])
	for _, change := range changes {
		if change.End > start {
			break
		}
		point += utf8.RuneCountInString(change.Text) - utf8.RuneCountInString(value[change.Start:change.End])
	}
	return point
}
