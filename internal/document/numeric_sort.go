package document

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

type SortMode uint8

const (
	SortIntegerAware SortMode = iota + 1
	SortDecimalComma
	SortDecimalDot
	SortLength
	SortLexicographic
	SortIgnoreCase
	SortReverse
	SortRandom
)

var decimalPrefix = regexp.MustCompile(`^[ \t\r\n]*-?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)`)

func (doc *Document) SortLinesMode(mode SortMode, ascending bool) (bool, error) {
	if mode < SortIntegerAware || mode > SortLength {
		return false, fmt.Errorf("unsupported line sort mode %d", mode)
	}
	block := doc.sourceOrderBlock()
	if len(block.lines) < 2 {
		return false, nil
	}
	lines := append([]string(nil), block.lines...)
	if mode == SortDecimalComma || mode == SortDecimalDot {
		type numericLine struct {
			line    string
			value   float64
			isEmpty bool
		}
		values := make([]numericLine, len(lines))
		for index, line := range lines {
			prepared := initialNumericCharacters(line, mode)
			values[index].line = line
			if strings.Trim(prepared, " \t\r\n") == "" {
				values[index].isEmpty = true
				continue
			}
			prefix := decimalPrefix.FindString(prepared)
			if prefix == "" {
				doc.replaceSortedLineBlock(block, block.lines)
				return false, fmt.Errorf("unable to sort number on line %d", strings.Count(doc.Editor.String()[:block.start], "\n")+index+1)
			}
			value, err := strconv.ParseFloat(strings.TrimSpace(prefix), 64)
			if err != nil {
				doc.replaceSortedLineBlock(block, block.lines)
				return false, fmt.Errorf("unable to sort number on line %d: %w", strings.Count(doc.Editor.String()[:block.start], "\n")+index+1, err)
			}
			values[index].value = value
		}
		sort.SliceStable(values, func(first, second int) bool {
			left, right := values[first], values[second]
			if left.isEmpty != right.isEmpty {
				return left.isEmpty == ascending
			}
			if ascending {
				return left.value < right.value
			}
			return left.value > right.value
		})
		for index, line := range values {
			lines[index] = line.line
		}
	} else {
		sort.SliceStable(lines, func(first, second int) bool {
			comparison := 0
			if mode == SortLength {
				comparison = utf16Length(lines[first]) - utf16Length(lines[second])
			} else {
				comparison = compareIntegerRuns(lines[first], lines[second])
			}
			if ascending {
				return comparison < 0
			}
			return comparison > 0
		})
	}
	return doc.replaceSortedLineBlock(block, lines), nil
}

func initialNumericCharacters(line string, mode SortMode) string {
	allowed := " \t\r\n0123456789-."
	if mode == SortDecimalComma {
		allowed = " \t\r\n0123456789-,"
	}
	index := strings.IndexFunc(line, func(character rune) bool { return !strings.ContainsRune(allowed, character) })
	if index >= 0 {
		line = line[:index]
	}
	if mode == SortDecimalComma {
		line = strings.ReplaceAll(line, ",", ".")
	}
	return line
}

func utf16Length(line string) int {
	length := 0
	for _, character := range line {
		length += utf16.RuneLen(character)
	}
	return length
}

func compareIntegerRuns(left, right string) int {
	first, second := utf16.Encode([]rune(left)), utf16.Encode([]rune(right))
	return compareIntegerUnits(first, second)
}

func compareIntegerUnits(first, second []uint16) int {
	for firstIndex, secondIndex := 0, 0; ; {
		if firstIndex == len(first) || secondIndex == len(second) {
			return slices.Compare(first[firstIndex:], second[secondIndex:])
		}
		firstNumeric, firstSign := numericRun(first, firstIndex)
		secondNumeric, secondSign := numericRun(second, secondIndex)
		if firstNumeric != secondNumeric {
			if first[firstIndex] != second[secondIndex] {
				return int(first[firstIndex]) - int(second[secondIndex])
			}
			firstIndex++
			secondIndex++
			continue
		}
		if firstNumeric {
			if firstSign != secondSign {
				return firstSign - secondSign
			}
			if firstSign < 0 {
				firstIndex++
				secondIndex++
			}
			firstEnd, secondEnd := firstIndex, secondIndex
			for firstEnd < len(first) && asciiDigit(first[firstEnd]) {
				firstEnd++
			}
			for secondEnd < len(second) && asciiDigit(second[secondEnd]) {
				secondEnd++
			}
			firstNumber, secondNumber := firstIndex, secondIndex
			for firstNumber < firstEnd && first[firstNumber] == '0' {
				firstNumber++
			}
			for secondNumber < secondEnd && second[secondNumber] == '0' {
				secondNumber++
			}
			if difference := (firstEnd - firstNumber) - (secondEnd - secondNumber); difference != 0 {
				return difference * firstSign
			}
			for offset := 0; offset < firstEnd-firstNumber; offset++ {
				if first[firstNumber+offset] != second[secondNumber+offset] {
					return (int(first[firstNumber+offset]) - int(second[secondNumber+offset])) * firstSign
				}
			}
			if difference := (secondNumber - secondIndex) - (firstNumber - firstIndex); difference != 0 {
				return difference
			}
			firstIndex, secondIndex = firstEnd, secondEnd
			continue
		}
		firstEnd, secondEnd := firstIndex, secondIndex
		for firstEnd < len(first) && !asciiDigit(first[firstEnd]) && first[firstEnd] != '-' {
			firstEnd++
		}
		for secondEnd < len(second) && !asciiDigit(second[secondEnd]) && second[secondEnd] != '-' {
			secondEnd++
		}
		if firstEnd == firstIndex {
			firstEnd++
		}
		if secondEnd == secondIndex {
			secondEnd++
		}
		if comparison := slices.Compare(first[firstIndex:firstEnd], second[secondIndex:secondEnd]); comparison != 0 {
			return comparison
		}
		firstIndex, secondIndex = firstEnd, secondEnd
	}
}

func numericRun(value []uint16, index int) (bool, int) {
	if asciiDigit(value[index]) {
		return true, 1
	}
	if value[index] == '-' && index+1 < len(value) && asciiDigit(value[index+1]) {
		return true, -1
	}
	return false, 1
}

func asciiDigit(character uint16) bool { return character >= '0' && character <= '9' }
