package document

import (
	"slices"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/vivaui/viva/text"
)

func (doc *Document) GoToLine(line int) bool {
	if line < 1 || line > doc.LineCount() {
		return false
	}
	if line == 1 {
		doc.Editor.MoveTo(0, false)
		return true
	}
	current, offset := 1, 0
	for _, character := range doc.Editor.String() {
		offset++
		if character == '\n' {
			current++
			if current == line {
				doc.Editor.MoveTo(offset, false)
				return true
			}
		}
	}
	return false
}

func (doc *Document) DuplicateLine() { doc.Editor.DuplicateLines(false) }

func (doc *Document) DeleteLine() { doc.Editor.DeleteLines() }

func (doc *Document) InsertBlankLineAbove() {
	value := doc.Editor.String()
	caret := doc.Editor.ByteOffset(doc.Editor.Caret())
	start := strings.LastIndexByte(value[:caret], '\n') + 1
	insertAt := start
	if start > 0 {
		insertAt--
	}
	doc.insertLineTarget(insertAt, doc.Editor.RuneIndex(start))
}

func (doc *Document) InsertBlankLineBelow() {
	value := doc.Editor.String()
	caret := doc.Editor.ByteOffset(doc.Editor.Caret())
	insertAt := len(value)
	if newline := strings.IndexByte(value[caret:], '\n'); newline >= 0 {
		insertAt = caret + newline + 1
	}
	position := doc.Editor.RuneIndex(insertAt)
	if insertAt == len(value) {
		position++
	}
	doc.insertLineTarget(insertAt, position)
}

func (doc *Document) lineOptions() text.CodeOptions {
	if options, ok := doc.Editor.Code(); ok {
		return options
	}
	return text.CodeOptions{TabWidth: 4, HardTabs: true}
}

func (doc *Document) IndentLines() {
	doc.Editor.InsertTabCode(doc.lineOptions())
}

func (doc *Document) UnindentLines() {
	doc.Editor.OutdentCode(doc.lineOptions())
}

type lineBlock struct {
	start, end int
	before     string
	lines      []string
	terminated bool
}

func (doc *Document) selectedLineBlock() lineBlock {
	value := doc.Editor.String()
	start, end := 0, len(value)
	selection := doc.Editor.Selection()
	if selection.Start != selection.End {
		first := doc.Editor.ByteOffset(selection.Start)
		last := doc.Editor.ByteOffset(selection.End - 1)
		start = strings.LastIndexByte(value[:first], '\n') + 1
		if newline := strings.IndexByte(value[last:], '\n'); newline >= 0 {
			end = last + newline + 1
		}
	}
	before := value[start:end]
	terminated := strings.HasSuffix(before, "\n")
	content := before
	if terminated {
		content = content[:len(content)-1]
	}
	return lineBlock{start: start, end: end, before: before, lines: strings.Split(content, "\n"), terminated: terminated}
}

func (block lineBlock) joined(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	value := strings.Join(lines, "\n")
	if block.terminated {
		value += "\n"
	}
	return value
}

func (doc *Document) SortLines(ascending bool) bool {
	return doc.sortLines(ascending, false)
}

func (doc *Document) SortLinesIgnoreCase(ascending bool) bool {
	return doc.sortLines(ascending, true)
}

func (doc *Document) ReverseLines() bool {
	block := doc.sourceOrderBlock()
	if len(block.lines) < 2 {
		return false
	}
	reversed := append([]string(nil), block.lines...)
	for first, last := 0, len(reversed)-1; first < last; first, last = first+1, last-1 {
		reversed[first], reversed[last] = reversed[last], reversed[first]
	}
	return doc.replaceSortedLineBlock(block, reversed)
}

func (doc *Document) JoinLines() bool {
	selection := doc.Editor.Selection()
	if selection.Start == selection.End {
		return false
	}
	block := doc.selectedLineBlock()
	if len(block.lines) < 2 {
		return false
	}
	after := doc.lineContentRange()
	changes := blankJoinTargets(doc.Editor.String(), doc.Editor.ByteOffset(after.Start), doc.Editor.ByteOffset(after.End))
	batch, err := doc.PrepareTargetEdits(changes)
	if err != nil || len(changes) == 0 {
		return false
	}
	batch.mapSelection(after)
	return batch.Apply() == nil
}

func (doc *Document) sortLines(ascending, ignoreCase bool) bool {
	block := doc.sourceOrderBlock()
	if len(block.lines) < 2 {
		return false
	}
	type keyedLine struct {
		value string
		key   []uint16
	}
	keyed := make([]keyedLine, len(block.lines))
	for index, line := range block.lines {
		key := line
		if ignoreCase {
			key = strings.ToLower(key)
		}
		keyed[index] = keyedLine{value: line, key: utf16.Encode([]rune(key))}
	}
	sort.SliceStable(keyed, func(first, second int) bool {
		comparison := slices.Compare(keyed[first].key, keyed[second].key)
		if ascending {
			return comparison < 0
		}
		return comparison > 0
	})
	sorted := make([]string, len(keyed))
	for index, line := range keyed {
		sorted[index] = line.value
	}
	return doc.replaceSortedLineBlock(block, sorted)
}

func (doc *Document) RemoveDuplicateLines() int {
	block := doc.sourceOrderBlock()
	if len(block.lines) < 2 {
		return 0
	}
	seen := make(map[string]bool, len(block.lines))
	kept := make([]string, 0, len(block.lines))
	removed := 0
	for _, line := range block.lines {
		if seen[line] {
			removed++
			continue
		}
		seen[line] = true
		kept = append(kept, line)
	}
	if removed != 0 && !doc.replaceSourceLineBlock(block, block.joined(kept), nil) {
		return 0
	}
	return removed
}
