package document

import (
	"math/rand/v2"
	"strings"
	"unicode"

	"github.com/vivaui/viva/text"
)

type CaseMode uint8

const (
	CaseUpper CaseMode = iota + 1
	CaseLower
	CaseInvert
	CaseProperForce
	CaseProperBlend
	CaseSentenceForce
	CaseSentenceBlend
	CaseRandom
)

func (doc *Document) ConvertCase(mode CaseMode) bool {
	selection := doc.Editor.Selection()
	if selection.Start == selection.End {
		return false
	}
	selected := []rune(doc.Editor.String())[selection.Start:selection.End]
	before := string(selected)
	var after string
	switch mode {
	case CaseUpper:
		after = strings.ToUpper(before)
	case CaseLower:
		after = strings.ToLower(before)
	case CaseInvert:
		for index, character := range selected {
			switch {
			case unicode.IsUpper(character) || unicode.IsTitle(character):
				selected[index] = unicode.ToLower(character)
			case unicode.IsLower(character):
				selected[index] = unicode.ToUpper(character)
			}
		}
		after = string(selected)
	case CaseProperForce, CaseProperBlend, CaseSentenceForce, CaseSentenceBlend, CaseRandom:
		after = transformCase(selected, mode, func() bool { return rand.IntN(2) == 1 })
	default:
		return false
	}
	if after == before {
		return false
	}
	doc.Editor.ReplaceRange(text.Range{Start: selection.Start, End: selection.End}, after)
	return true
}

func transformCase(characters []rune, mode CaseMode, upper func() bool) string {
	switch mode {
	case CaseProperForce, CaseProperBlend:
		for index, character := range characters {
			if !unicode.IsLetter(character) {
				continue
			}
			quoted := index >= 2 && singleQuote(characters[index-1]) && caseAlphanumeric(characters[index-2])
			if !quoted && (index == 0 || !caseAlphanumeric(characters[index-1])) {
				characters[index] = unicode.ToUpper(character)
			} else if mode == CaseProperForce {
				characters[index] = unicode.ToLower(character)
			}
		}
	case CaseSentenceForce, CaseSentenceBlend:
		newSentence, wasCR, wasLF := true, false, false
		for index, character := range characters {
			switch {
			case unicode.IsLetter(character):
				if newSentence {
					characters[index] = unicode.ToUpper(character)
					newSentence = false
				} else if mode == CaseSentenceForce {
					characters[index] = unicode.ToLower(character)
				}
				wasCR, wasLF = false, false
				if characters[index] == 'i' && index > 0 && index+1 < len(characters) &&
					(unicode.IsSpace(characters[index-1]) || characters[index-1] == '(' || characters[index-1] == '"') &&
					(unicode.IsSpace(characters[index+1]) || characters[index+1] == '\'') {
					characters[index] = 'I'
				}
			case character == '.' || character == '!' || character == '?':
				newSentence = index+1 == len(characters) || !caseAlphanumeric(characters[index+1])
			case character == '\r':
				if wasCR {
					newSentence = true
				} else {
					wasCR = true
				}
			case character == '\n':
				if wasLF {
					newSentence = true
				} else {
					wasLF = true
				}
			}
		}
	case CaseRandom:
		for index, character := range characters {
			if !unicode.IsLetter(character) {
				continue
			}
			if upper() {
				characters[index] = unicode.ToUpper(character)
			} else {
				characters[index] = unicode.ToLower(character)
			}
		}
	}
	return string(characters)
}

func caseAlphanumeric(character rune) bool {
	return unicode.IsLetter(character) || unicode.IsNumber(character)
}

func singleQuote(character rune) bool {
	return character == '\'' || character == '\u2018' || character == '\u2019'
}
