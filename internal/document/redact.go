package document

import (
	"fmt"

	"github.com/vivaui/viva/text"
)

func (doc *Document) RedactRanges(ranges []text.Range, bullet bool) (bool, error) {
	characters := []rune(doc.Editor.String())
	mask := '█'
	if bullet {
		mask = '●'
	}
	if doc.Encoding().IsLegacy() {
		mask = '#'
		if bullet {
			mask = '.'
		}
	}
	edits := make([]text.Replacement, 0, len(ranges))
	previous := 0
	changed := false
	for index, selection := range ranges {
		if selection.Start < previous || selection.End < selection.Start || selection.End > len(characters) {
			return false, fmt.Errorf("invalid or overlapping redaction range %d", index)
		}
		previous = selection.End
		if selection.Start == selection.End {
			continue
		}
		replacement := make([]rune, selection.End-selection.Start)
		for offset, character := range characters[selection.Start:selection.End] {
			replacement[offset] = mask
			if character == '\r' || character == '\n' {
				replacement[offset] = character
			} else if character != mask {
				changed = true
			}
		}
		edits = append(edits, text.Replacement{Range: selection, Text: string(replacement)})
	}
	if !changed {
		return false, nil
	}
	if err := doc.Editor.ReplaceRanges(edits); err != nil {
		return false, err
	}
	return true, nil
}
