package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"vivapad/internal/document"
)

type windowSortCommand struct {
	label   string
	column  int
	reverse bool
}

var windowSortCommands = [...]windowSortCommand{
	{"Name A to Z", 0, false}, {"Name Z to A", 0, true},
	{"Path A to Z", 1, false}, {"Path Z to A", 1, true},
	{"Type A to Z", 2, false}, {"Type Z to A", 2, true},
	{"Content Length Ascending", 3, false}, {"Content Length Descending", 3, true},
	{"Modified Time Ascending", 4, false}, {"Modified Time Descending", 4, true},
}

func windowSortFor(command string) (windowSortCommand, bool) {
	for _, spec := range windowSortCommands {
		if spec.label == command {
			return spec, true
		}
	}
	return windowSortCommand{}, false
}

func lowerASCII(character byte) byte {
	if character >= 'A' && character <= 'Z' {
		return character + ('a' - 'A')
	}
	return character
}

func windowSortCodeUnits(value string) (first, second rune, width int) {
	if value[0] < utf8.RuneSelf {
		return rune(lowerASCII(value[0])), 0, 1
	}
	character, width := utf8.DecodeRuneInString(value)
	if character == utf8.RuneError && width == 1 {
		// Preserve distinct invalid bytes in Unix filenames as lone low surrogates.
		return 0xDC00 + rune(value[0]), 0, width
	}
	if character <= 0xFFFF {
		return character, 0, width
	}
	first, second = utf16.EncodeRune(character)
	return first, second, width
}

func windowNaturalCompare(first, second string) int {
	left, right := 0, 0
	for left < len(first) && right < len(second) {
		if first[left] >= '0' && first[left] <= '9' && second[right] >= '0' && second[right] <= '9' {
			leftEnd, rightEnd := left, right
			for leftEnd < len(first) && first[leftEnd] >= '0' && first[leftEnd] <= '9' {
				leftEnd++
			}
			for rightEnd < len(second) && second[rightEnd] >= '0' && second[rightEnd] <= '9' {
				rightEnd++
			}
			leftValue, rightValue := strings.TrimLeft(first[left:leftEnd], "0"), strings.TrimLeft(second[right:rightEnd], "0")
			if len(leftValue) != len(rightValue) {
				return sign(len(leftValue) - len(rightValue))
			}
			if result := strings.Compare(leftValue, rightValue); result != 0 {
				return sign(result)
			}
			if leftEnd-left != rightEnd-right {
				return sign((rightEnd - right) - (leftEnd - left))
			}
			left, right = leftEnd, rightEnd
			continue
		}
		a, aNext, aWidth := windowSortCodeUnits(first[left:])
		b, bNext, bWidth := windowSortCodeUnits(second[right:])
		if a != b {
			return sign(int(a) - int(b))
		}
		if aNext != bNext {
			return sign(int(aNext) - int(bNext))
		}
		left += aWidth
		right += bWidth
	}
	return sign((len(first) - left) - (len(second) - right))
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

type windowSortKey struct {
	name, path, kind string
	length           int
	modified         time.Time
}

func windowContentLength(doc *document.Document) int {
	// At the end of the editor, ByteOffset reads the store's UTF-8 byte total.
	return doc.Editor.ByteOffset(doc.Editor.RuneCount())
}

func (state *model) windowKey(doc *document.Document, needsTime bool) (windowSortKey, error) {
	path := doc.Path
	if path == "" {
		path = doc.Name
	}
	key := windowSortKey{name: doc.Name, path: path, kind: state.languageLabel(doc), length: windowContentLength(doc)}
	if needsTime && doc.Path != "" {
		info, err := os.Stat(doc.Path)
		if err != nil {
			return windowSortKey{}, fmt.Errorf("%s: %w", doc.Path, err)
		}
		key.modified = info.ModTime()
	}
	return key, nil
}

func compareWindowKeys(first, second windowSortKey, column int) int {
	result := 0
	switch column {
	case 0:
		result = windowNaturalCompare(first.name, second.name)
	case 1:
		result = windowNaturalCompare(first.path, second.path)
	case 2:
		result = windowNaturalCompare(first.kind, second.kind)
	case 3:
		result = sign(first.length - second.length)
	case 4:
		if first.modified.Before(second.modified) {
			result = -1
		} else if first.modified.After(second.modified) {
			result = 1
		}
	}
	if result == 0 {
		return windowNaturalCompare(first.path, second.path)
	}
	return result
}

func (state *model) sortWindowTabs(command windowSortCommand) error {
	pane := state.activePane
	selected := state.space.Current()
	if state.splitEnabled {
		selected = state.paneDocument(pane)
	}
	var positions []int
	var tabs []*document.Document
	keys := make(map[*document.Document]windowSortKey)
	for index, doc := range state.space.Documents {
		if state.splitEnabled && state.paneOwner[doc] != pane {
			continue
		}
		key, err := state.windowKey(doc, command.column == 4)
		if err != nil {
			return err
		}
		positions = append(positions, index)
		tabs = append(tabs, doc)
		keys[doc] = key
	}
	slices.SortStableFunc(tabs, func(first, second *document.Document) int {
		if state.isPinned(first) != state.isPinned(second) {
			if state.isPinned(first) {
				return -1
			}
			return 1
		}
		result := compareWindowKeys(keys[first], keys[second], command.column)
		if command.reverse {
			result = -result
		}
		return result
	})
	for index, position := range positions {
		state.space.Documents[position] = tabs[index]
	}
	if index := state.documentIndex(selected); index >= 0 {
		state.space.Select(index)
	}
	state.paneTabAnchor[pane] = nil
	state.tabActive = -1
	return nil
}
