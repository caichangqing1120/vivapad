package document

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/vivaui/viva/text"
)

func (doc *Document) SortRectangle(rect Rectangle, mode SortMode, ascending bool, random *rand.Rand) (bool, error) {
	if mode < SortIntegerAware || mode > SortRandom || mode == SortRandom && random == nil {
		return false, fmt.Errorf("unsupported rectangular sort mode %d", mode)
	}
	rows, err := doc.rectangleRows(rect)
	if err != nil {
		return false, err
	}
	singleRow := len(rows) == 1
	if singleRow && rect.Anchor != rect.Caret {
		return false, nil
	}
	before := doc.Editor.String()
	lines := strings.Split(before, "\n")
	first, last := rows[0].Line, rows[len(rows)-1].Line
	if singleRow {
		first, last = 0, len(lines)-1
	}
	if first == last {
		return false, nil
	}
	start := 0
	for _, line := range lines[:first] {
		start += len(line) + 1
	}
	end := start
	for _, line := range lines[first : last+1] {
		end += len(line) + 1
	}
	end = min(end, len(before))
	block := lineBlock{start: start, end: end, before: before[start:end], lines: lines[first : last+1], terminated: last+1 < len(lines)}
	anchor, caret := doc.Editor.ByteOffset(rect.Anchor), doc.Editor.ByteOffset(rect.Caret)
	anchorColumn := anchor - strings.LastIndexByte(before[:anchor], '\n') - 1 + rect.AnchorVirtual
	caretColumn := caret - strings.LastIndexByte(before[:caret], '\n') - 1 + rect.CaretVirtual
	from, to := min(anchorColumn, caretColumn), max(anchorColumn, caretColumn)
	if singleRow {
		from, to = 0, 0
	}
	ordered := append([]string(nil), block.lines...)
	switch mode {
	case SortReverse:
		slices.Reverse(ordered)
	case SortRandom:
		random.Shuffle(len(ordered), func(first, second int) { ordered[first], ordered[second] = ordered[second], ordered[first] })
	default:
		if err := orderRectangleKeys(ordered, from, to, mode, ascending, first); err != nil {
			return false, err
		}
	}
	after := block.joined(ordered)
	if after == block.before {
		return false, nil
	}
	batch, err := doc.PrepareTargetEdits([]ReplacementEdit{{Start: start, End: end, Text: after}})
	if err != nil {
		return false, err
	}
	batch.mapSelection(text.Range{Start: rect.Caret, End: rect.Caret})
	if err := batch.Apply(); err != nil {
		return false, err
	}
	return true, nil
}

func orderRectangleKeys(lines []string, from, to int, mode SortMode, ascending bool, firstLine int) error {
	type keyedLine struct {
		line   string
		key    []uint16
		number float64
		empty  bool
	}
	keyed := make([]keyedLine, len(lines))
	for index, line := range lines {
		key := utf16.Encode([]rune(line))
		if to != 0 {
			end := len(key)
			if from != to {
				end = min(end, to)
			}
			key = key[min(from, len(key)):end]
		}
		if mode == SortIgnoreCase {
			for position, unit := range key {
				if unit == 0 {
					key = key[:position]
					break
				}
				key[position] = uint16(unicode.ToLower(rune(unit)))
			}
		}
		keyed[index] = keyedLine{line: line, key: key}
		if mode != SortDecimalDot && mode != SortDecimalComma {
			continue
		}
		prepared := initialNumericCharacters(string(utf16.Decode(key)), mode)
		if strings.Trim(prepared, " \t\r\n") == "" {
			keyed[index].empty = true
			continue
		}
		prefix := decimalPrefix.FindString(prepared)
		if prefix == "" {
			return fmt.Errorf("unable to sort number on line %d", firstLine+index+1)
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(prefix), 64)
		if err != nil {
			return fmt.Errorf("unable to sort number on line %d: %w", firstLine+index+1, err)
		}
		keyed[index].number = number
	}
	sort.SliceStable(keyed, func(first, second int) bool {
		left, right := keyed[first], keyed[second]
		comparison := 0
		switch mode {
		case SortIntegerAware:
			comparison = compareIntegerUnits(left.key, right.key)
		case SortLength:
			comparison = len(left.key) - len(right.key)
		case SortDecimalComma, SortDecimalDot:
			if left.empty != right.empty {
				return left.empty == ascending
			}
			if left.number < right.number {
				comparison = -1
			} else if left.number > right.number {
				comparison = 1
			}
		default:
			comparison = slices.Compare(left.key, right.key)
		}
		if ascending {
			return comparison < 0
		}
		return comparison > 0
	})
	for index, entry := range keyed {
		lines[index] = entry.line
	}
	return nil
}
