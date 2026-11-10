package document

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
)

type Rectangle struct {
	Anchor, Caret               int
	AnchorVirtual, CaretVirtual int
	TabWidth                    int
}

type RectangleRow struct {
	Line                    int
	Range                   text.Range
	LeftPadding             int
	VirtualCells            int
	LeftColumn, RightColumn int
}

func (doc *Document) RectangleRows(rect Rectangle) []RectangleRow {
	rows, _ := doc.rectangleRows(rect)
	return rows
}

func (doc *Document) RectangleText(rect Rectangle) (string, error) {
	rows, err := doc.rectangleRows(rect)
	if err != nil {
		return "", err
	}
	width := rows[0].RightColumn - rows[0].LeftColumn
	if width == 0 {
		return "", nil
	}
	characters := []rune(doc.Editor.String())
	var output strings.Builder
	for _, row := range rows {
		output.WriteString(string(characters[row.Range.Start:row.Range.End]))
		if len(rows) > 1 {
			output.WriteString(doc.newline)
		}
		if output.Len() > 16<<20 {
			return "", fmt.Errorf("column clipboard text exceeds 16 MiB")
		}
	}
	return output.String(), nil
}

func (doc *Document) rectangleRows(rect Rectangle) ([]RectangleRow, error) {
	if rect.Anchor < 0 || rect.Anchor > doc.Editor.RuneCount() || rect.Caret < 0 || rect.Caret > doc.Editor.RuneCount() ||
		rect.AnchorVirtual < 0 || rect.CaretVirtual < 0 || rect.AnchorVirtual > 1_000_000 || rect.CaretVirtual > 1_000_000 || rect.TabWidth < 1 || rect.TabWidth > 16 {
		return nil, fmt.Errorf("invalid rectangular selection")
	}
	value := doc.Editor.String()
	anchorByte, caretByte := doc.Editor.ByteOffset(rect.Anchor), doc.Editor.ByteOffset(rect.Caret)
	anchorLine, caretLine := strings.Count(value[:anchorByte], "\n"), strings.Count(value[:caretByte], "\n")
	anchorStart := strings.LastIndexByte(value[:anchorByte], '\n') + 1
	caretStart := strings.LastIndexByte(value[:caretByte], '\n') + 1
	anchorColumn := columnCells(value[anchorStart:anchorByte], rect.TabWidth) + rect.AnchorVirtual
	caretColumn := columnCells(value[caretStart:caretByte], rect.TabWidth) + rect.CaretVirtual
	left, right := min(anchorColumn, caretColumn), max(anchorColumn, caretColumn)
	first, last := min(anchorLine, caretLine), max(anchorLine, caretLine)
	lines := strings.Split(value, "\n")
	rows := make([]RectangleRow, 0, last-first+1)
	lineByte := 0
	for line, content := range lines {
		if line >= first && line <= last {
			leftOffset, leftWidth := rectangleOffsetNearest(content, left, rect.TabWidth)
			rightOffset, _ := rectangleOffsetNearest(content, right, rect.TabWidth)
			padding := 0
			if leftOffset == len(content) {
				padding = max(0, left-leftWidth)
			}
			rows = append(rows, RectangleRow{
				Line:        line,
				Range:       text.Range{Start: doc.Editor.RuneIndex(lineByte + leftOffset), End: doc.Editor.RuneIndex(lineByte + rightOffset)},
				LeftPadding: padding, VirtualCells: padding,
				LeftColumn: left, RightColumn: right,
			})
		}
		lineByte += len(content) + 1
	}
	return rows, nil
}

func rectangleOffsetNearest(value string, target, tabWidth int) (int, int) {
	column := 0
	for offset, character := range value {
		width := text.CellWidth(character)
		if character == '\t' {
			width = tabWidth - column%tabWidth
		}
		if width == 0 {
			continue
		}
		if target <= column || target < column+width && target*2 < 2*column+width {
			return offset, column
		}
		column += width
		if target < column {
			return offset + utf8.RuneLen(character), column
		}
	}
	return len(value), column
}

func (doc *Document) EditRectangle(rect Rectangle, options ColumnEdit) (bool, error) {
	rows, err := doc.rectangleRows(rect)
	if err != nil {
		return false, err
	}
	insertions, err := columnInsertions(options, len(rows))
	if err != nil {
		return false, err
	}
	edits := make([]text.Replacement, 0, len(rows))
	for index, row := range rows {
		edits = append(edits, text.Replacement{Range: row.Range, Text: strings.Repeat(" ", row.LeftPadding) + insertions[index]})
	}
	before := doc.Editor.String()
	if err := doc.Editor.ReplaceRanges(edits); err != nil {
		return false, err
	}
	return doc.Editor.String() != before, nil
}

func (doc *Document) PasteRectangleLines(rect Rectangle, pasted string) (bool, error) {
	rows, err := doc.rectangleRows(rect)
	if err != nil {
		return false, err
	}
	if len(pasted) > 16<<20 {
		return false, fmt.Errorf("column paste exceeds 16 MiB")
	}
	normalized, _, _ := normalizeLineEndings(pasted)
	normalized = strings.TrimRight(normalized, "\n")
	fragments := []string(nil)
	if normalized != "" {
		fragments = strings.Split(normalized, "\n")
	}
	before := doc.Editor.String()
	lines := strings.Split(before, "\n")
	first, last := rows[0].Line, rows[len(rows)-1].Line
	left := rows[0].LeftColumn
	edits := make([]text.Replacement, 0, max(len(rows), len(fragments)))
	lineByte := 0
	for line, content := range lines {
		fragmentIndex := line - first
		if line >= first && (line <= last || fragmentIndex < len(fragments)) {
			start, end := 0, 0
			padding := 0
			if line <= last {
				row := rows[fragmentIndex]
				start, end, padding = row.Range.Start, row.Range.End, row.LeftPadding
			} else {
				offset, width := rectangleOffsetNearest(content, left, rect.TabWidth)
				start = doc.Editor.RuneIndex(lineByte + offset)
				end = start
				if offset == len(content) {
					padding = max(0, left-width)
				}
			}
			inserted := ""
			if fragmentIndex < len(fragments) {
				inserted = strings.Repeat(" ", padding) + fragments[fragmentIndex]
			}
			if start != end || inserted != "" {
				edits = append(edits, text.Replacement{Range: text.Range{Start: start, End: end}, Text: inserted})
			}
		}
		lineByte += len(content) + 1
	}
	if extra := len(lines) - first; extra < len(fragments) {
		var appended strings.Builder
		for _, fragment := range fragments[extra:] {
			appended.WriteByte('\n')
			appended.WriteString(strings.Repeat(" ", left))
			appended.WriteString(fragment)
		}
		eof := doc.Editor.RuneCount()
		if len(edits) > 0 && edits[len(edits)-1].Range.Start == eof {
			edits[len(edits)-1].Text += appended.String()
		} else {
			edits = append(edits, text.Replacement{Range: text.Range{Start: eof, End: eof}, Text: appended.String()})
		}
	}
	if len(edits) == 0 {
		return false, nil
	}
	if err := doc.Editor.ReplaceRanges(edits); err != nil {
		return false, err
	}
	return doc.Editor.String() != before, nil
}

func (doc *Document) DeleteRectangle(rect Rectangle, backward bool) (bool, error) {
	rows, err := doc.rectangleRows(rect)
	if err != nil {
		return false, err
	}
	selected := rows[0].LeftColumn != rows[0].RightColumn
	value := doc.Editor.String()
	lines := strings.Split(value, "\n")
	edits := make([]text.Replacement, 0, len(rows))
	lineStart := 0
	for line, content := range lines {
		if line >= rows[0].Line && line <= rows[len(rows)-1].Line {
			row := rows[line-rows[0].Line]
			target := row.Range
			if target.Start == target.End && !selected && row.VirtualCells == 0 {
				local := target.Start - doc.Editor.RuneIndex(lineStart)
				cursor := text.NewEditor(content)
				cursor.MoveTo(local, false)
				if backward {
					cursor.MoveLeft(false)
					target.Start = doc.Editor.RuneIndex(lineStart) + cursor.Caret()
				} else {
					cursor.MoveRight(false)
					target.End = doc.Editor.RuneIndex(lineStart) + cursor.Caret()
				}
			}
			if target.Start != target.End {
				edits = append(edits, text.Replacement{Range: target})
			}
		}
		lineStart += len(content) + 1
	}
	if len(edits) == 0 {
		return false, nil
	}
	if err := doc.Editor.ReplaceRanges(edits); err != nil {
		return false, err
	}
	return true, nil
}
