package document

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"golang.org/x/text/language"
)

func (doc *Document) SortLinesLocale(locale language.Tag, ascending bool) bool {
	accepted, _ := doc.SortLinesLocaleWithError(locale, ascending)
	return accepted
}

func (doc *Document) SortLinesLocaleWithError(locale language.Tag, ascending bool) (bool, error) {
	return doc.sortLinesLocaleKeys(newLocaleKeys(locale), ascending)
}

func (doc *Document) sortLinesLocaleKeys(keys localeKeys, ascending bool) (bool, error) {
	block := doc.selectedLineBlock()
	if len(block.lines) < 2 {
		return false, nil
	}
	type localeLine struct {
		value string
		index int
		key   []byte
	}
	ordered := make([]localeLine, len(block.lines))
	for index, line := range block.lines {
		key, err := keys.key(line)
		if err != nil {
			return false, fmt.Errorf("Locale sort key on line %d: %w", strings.Count(doc.Editor.String()[:block.start], "\n")+index+1, err)
		}
		ordered[index] = localeLine{value: line, index: index, key: key}
	}
	sort.SliceStable(ordered, func(first, second int) bool {
		comparison := compareLocaleKeys(ordered[first].key, ordered[second].key)
		if ascending {
			return comparison < 0
		}
		return comparison > 0
	})
	lines := make([]string, len(ordered))
	for index, line := range ordered {
		lines[index] = line.value
	}
	value := block.joined(lines)
	editor := doc.Editor
	selection := editor.Selection()
	start := editor.RuneIndex(block.start)
	after := text.Range{Start: start, End: start + utf8.RuneCountInString(value)}
	if selection.Start == selection.End {
		before := editor.String()
		caretByte := editor.ByteOffset(editor.Caret())
		oldLine := strings.Count(before[:caretByte], "\n")
		lineStart := strings.LastIndexByte(before[:caretByte], '\n') + 1
		column := editor.Caret() - editor.RuneIndex(lineStart)
		position := start
		for _, line := range ordered {
			if line.index == oldLine {
				break
			}
			position += utf8.RuneCountInString(line.value) + 1
		}
		if oldLine >= len(ordered) {
			position = start + utf8.RuneCountInString(value)
		} else {
			position += column
		}
		after = text.Range{Start: position, End: position}
	} else if editor.Caret() == selection.Start {
		after = text.Range{Start: after.End, End: after.Start}
	}
	batch, err := doc.PrepareTargetEdits([]ReplacementEdit{{Start: block.start, End: block.end, Text: value}})
	if err != nil {
		return false, err
	}
	batch.after = after
	if err := batch.Apply(); err != nil {
		return false, err
	}
	return true, nil
}

// randomizeBlock is the line block RandomizeLines replaces: the source
// selection without sourceOrderBlock's synthetic trailing empty line, so a
// single-line document has nothing to permute.
func (doc *Document) randomizeBlock() lineBlock {
	return doc.selectedLineBlock()
}

// RandomizeLineTarget reports the inclusive line range that RandomizeLines
// would replace when the operation is applicable. It lets callers apply a
// target-scoped marker policy when a shuffle happens to reproduce the source
// order (no edit is recorded, so no edit-driven marker mapping happens).
func (doc *Document) RandomizeLineTarget() (start, end int, ok bool) {
	block := doc.randomizeBlock()
	if len(block.lines) < 2 {
		return 0, 0, false
	}
	value := doc.Editor.String()
	if block.start < 0 || block.start > len(value) {
		return 0, 0, false
	}
	start = strings.Count(value[:block.start], "\n")
	end = start + len(block.lines) - 1
	if total := strings.Count(value, "\n"); end > total {
		end = total
	}
	return start, end, true
}

// RandomizeLinesOutcome reports whether the randomize changed the text, whether
// it was applicable at all, and the refusal cause. applicable is true exactly
// when an eligible block was replaced, even if the shuffle reproduced the order;
// a refused edit returns applicable=false with the error.
func (doc *Document) RandomizeLinesOutcome(random *rand.Rand) (changed, applicable bool, err error) {
	if random == nil {
		return false, false, nil
	}
	block := doc.randomizeBlock()
	if len(block.lines) < 2 {
		return false, false, nil
	}
	lines := append([]string(nil), block.lines...)
	random.Shuffle(len(lines), func(first, second int) { lines[first], lines[second] = lines[second], lines[first] })
	changed, err = doc.replaceSortedLineBlockOutcome(block, lines)
	if err != nil {
		return false, false, err
	}
	return changed, true, nil
}

func (doc *Document) RandomizeLines(random *rand.Rand) bool {
	changed, _, _ := doc.RandomizeLinesOutcome(random)
	return changed
}
