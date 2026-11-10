package document

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"golang.org/x/text/language"
)

func (doc *Document) SortRectangleLocale(rect Rectangle, locale language.Tag, ascending bool) (Rectangle, bool, error) {
	return doc.sortRectangleLocaleKeys(rect, newLocaleKeys(locale), ascending)
}

func (doc *Document) sortRectangleLocaleKeys(rect Rectangle, keys localeKeys, ascending bool) (Rectangle, bool, error) {
	rows, err := doc.rectangleRows(rect)
	if err != nil {
		return rect, false, err
	}
	if len(rows) < 2 {
		return rect, false, nil
	}
	before := doc.Editor.String()
	lines := strings.Split(before, "\n")
	starts := make([]int, len(lines))
	for index := 1; index < len(lines); index++ {
		starts[index] = starts[index-1] + utf8.RuneCountInString(lines[index-1]) + 1
	}
	first, last := rows[0].Line, rows[len(rows)-1].Line
	start := doc.Editor.ByteOffset(starts[first])
	end := len(before)
	if last+1 < len(lines) {
		end = doc.Editor.ByteOffset(starts[last+1])
	}
	empty := true
	for _, row := range rows {
		rightVirtual := max(0, row.RightColumn-columnCells(lines[row.Line], rect.TabWidth))
		if row.Range.Start != row.Range.End || row.LeftPadding != rightVirtual {
			empty = false
			break
		}
	}
	anchorByte, caretByte := doc.Editor.ByteOffset(rect.Anchor), doc.Editor.ByteOffset(rect.Caret)
	anchorStart := strings.LastIndexByte(before[:anchorByte], '\n') + 1
	caretStart := strings.LastIndexByte(before[:caretByte], '\n') + 1
	anchorColumn := columnCells(before[anchorStart:anchorByte], rect.TabWidth) + rect.AnchorVirtual
	caretColumn := columnCells(before[caretStart:caretByte], rect.TabWidth) + rect.CaretVirtual
	anchorLeft := anchorColumn <= caretColumn
	type localeRow struct {
		content                     string
		key                         []byte
		anchor, caret               int
		anchorVirtual, caretVirtual int
	}
	ordered := make([]localeRow, len(rows))
	for index, row := range rows {
		keyEnd := row.Range.End
		if empty {
			keyEnd = starts[row.Line] + utf8.RuneCountInString(lines[row.Line])
		}
		key, err := keys.key(before[doc.Editor.ByteOffset(row.Range.Start):doc.Editor.ByteOffset(keyEnd)])
		if err != nil {
			return rect, false, fmt.Errorf("Locale sort key on line %d: %w", row.Line+1, err)
		}
		entry := localeRow{
			content: lines[row.Line], key: key,
			anchor: row.Range.Start - starts[row.Line], caret: row.Range.End - starts[row.Line],
			anchorVirtual: row.LeftPadding, caretVirtual: max(0, row.RightColumn-columnCells(lines[row.Line], rect.TabWidth)),
		}
		if !anchorLeft {
			entry.anchor, entry.caret = entry.caret, entry.anchor
			entry.anchorVirtual, entry.caretVirtual = entry.caretVirtual, entry.anchorVirtual
		}
		ordered[index] = entry
	}
	sort.SliceStable(ordered, func(first, second int) bool {
		left, right := ordered[first].key, ordered[second].key
		comparison := compareLocaleKeys(left, right)
		if ascending {
			return comparison < 0
		}
		return comparison > 0
	})
	contents := make([]string, len(ordered))
	for index, row := range ordered {
		contents[index] = row.content
	}
	value := strings.Join(contents, "\n")
	if last+1 < len(lines) {
		value += "\n"
	}
	firstStart, lastStart := starts[first], starts[first]
	for _, row := range ordered[:len(ordered)-1] {
		lastStart += utf8.RuneCountInString(row.content) + 1
	}
	anchorRow, caretRow := ordered[0], ordered[len(ordered)-1]
	if rect.Caret < rect.Anchor {
		anchorRow, caretRow = caretRow, anchorRow
		firstStart, lastStart = lastStart, firstStart
	}
	after := Rectangle{
		Anchor: firstStart + anchorRow.anchor, Caret: lastStart + caretRow.caret,
		AnchorVirtual: anchorRow.anchorVirtual, CaretVirtual: caretRow.caretVirtual, TabWidth: rect.TabWidth,
	}
	batch, err := doc.PrepareTargetEdits([]ReplacementEdit{{Start: start, End: end, Text: value}})
	if err != nil {
		return rect, false, err
	}
	batch.after = text.Range{Start: after.Caret, End: after.Caret}
	if err := batch.Apply(); err != nil {
		return rect, false, err
	}
	return after, true, nil
}
