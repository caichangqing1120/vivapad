package document

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/vivaui/viva/text"
)

type ColumnLeading uint8

const (
	ColumnLeadingNone ColumnLeading = iota
	ColumnLeadingZero
	ColumnLeadingSpace
)

type ColumnEdit struct {
	Text                       string
	Number                     bool
	Initial, Increment, Repeat string
	Base                       int
	Uppercase                  bool
	Leading                    ColumnLeading
}

func (doc *Document) EditColumn(options ColumnEdit, tabWidth int) (bool, error) {
	if tabWidth < 1 || tabWidth > 16 {
		return false, fmt.Errorf("invalid tab width")
	}
	value := doc.Editor.String()
	caret := doc.Editor.ByteOffset(doc.Editor.Caret())
	start := strings.LastIndexByte(value[:caret], '\n') + 1
	column := columnCells(value[start:caret], tabWidth)
	lines := strings.Split(value[start:], "\n")
	insertions, err := columnInsertions(options, len(lines))
	if err != nil {
		return false, err
	}
	edits := make([]ReplacementEdit, 0, len(lines))
	lineStart, changed := start, false
	for index, line := range lines {
		offset, width := columnOffset(line, column, tabWidth)
		after := line[:offset] + strings.Repeat(" ", max(0, column-width)) + insertions[index] + line[offset:]
		changed = changed || after != line
		edits = append(edits, ReplacementEdit{Start: lineStart, End: lineStart + len(line), Text: after})
		lineStart += len(line) + 1
	}
	if !changed {
		return false, nil
	}
	if err := doc.applyTargetEdits(edits); err != nil {
		return false, err
	}
	return true, nil
}

func columnInsertions(options ColumnEdit, count int) ([]string, error) {
	if !options.Number {
		if options.Text == "" || len(options.Text) > 4096 || strings.IndexAny(options.Text, "\r\n\x00") >= 0 {
			return nil, fmt.Errorf("invalid column text")
		}
		insertions := make([]string, count)
		for index := range insertions {
			insertions[index] = options.Text
		}
		return insertions, nil
	}
	if options.Base != 2 && options.Base != 8 && options.Base != 10 && options.Base != 16 || options.Leading > ColumnLeadingSpace {
		return nil, fmt.Errorf("invalid column number format")
	}
	initial, err := strconv.ParseInt(options.Initial, options.Base, 32)
	if err != nil || initial < 0 {
		return nil, fmt.Errorf("invalid initial column number")
	}
	increment, err := strconv.ParseInt(options.Increment, options.Base, 32)
	if err != nil || increment < 0 {
		return nil, fmt.Errorf("invalid column increment")
	}
	repeat, err := strconv.ParseInt(options.Repeat, options.Base, 32)
	if err != nil || repeat <= 0 {
		return nil, fmt.Errorf("invalid column repeat")
	}
	step := int64(count-1) / repeat
	if increment > 0 && step > (math.MaxInt32-initial)/increment {
		return nil, fmt.Errorf("column number overflow")
	}
	last := initial + step*increment
	width := max(len(strconv.FormatInt(initial, options.Base)), len(strconv.FormatInt(last, options.Base)))
	insertions := make([]string, count)
	for index := range insertions {
		value := strconv.FormatInt(initial+int64(index)/repeat*increment, options.Base)
		if options.Uppercase && options.Base == 16 {
			value = strings.ToUpper(value)
		}
		if options.Leading != ColumnLeadingNone {
			padding := "0"
			if options.Leading == ColumnLeadingSpace {
				padding = " "
			}
			value = strings.Repeat(padding, width-len(value)) + value
		}
		insertions[index] = value
	}
	return insertions, nil
}

func columnCells(value string, tabWidth int) int {
	column := 0
	for _, character := range value {
		if character == '\t' {
			column += tabWidth - column%tabWidth
		} else {
			column += text.CellWidth(character)
		}
	}
	return column
}

func columnOffset(value string, target, tabWidth int) (int, int) {
	column := 0
	for offset, character := range value {
		if column >= target && text.CellWidth(character) != 0 {
			return offset, column
		}
		if character == '\t' {
			column += tabWidth - column%tabWidth
		} else {
			column += text.CellWidth(character)
		}
	}
	return len(value), column
}
