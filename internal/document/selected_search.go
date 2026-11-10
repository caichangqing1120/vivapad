package document

import (
	"unicode"

	"github.com/vivaui/viva/text"
)

func (doc *Document) SelectedSearchText(expand bool) string {
	selection := doc.Editor.Selection()
	if selection.Start == selection.End && expand {
		characters := []rune(doc.Editor.String())
		caret := doc.Editor.Caret()
		at := -1
		if caret < len(characters) && selectedWordRune(characters[caret]) {
			at = caret
		} else if caret > 0 && selectedWordRune(characters[caret-1]) {
			at = caret - 1
		}
		if at >= 0 {
			first, last := at, at+1
			for first > 0 && selectedWordRune(characters[first-1]) {
				first--
			}
			for last < len(characters) && selectedWordRune(characters[last]) {
				last++
			}
			selection = text.Range{Start: first, End: last}
			doc.Editor.SetSelection(selection)
		}
	}
	if selection.Start == selection.End {
		return ""
	}
	value := doc.Editor.String()
	return value[doc.Editor.ByteOffset(selection.Start):doc.Editor.ByteOffset(selection.End)]
}

func selectedWordRune(character rune) bool {
	return character == '_' || unicode.IsLetter(character) || unicode.IsDigit(character) || unicode.IsMark(character)
}
