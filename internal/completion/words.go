package completion

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxWordBytes = 255

type Result struct {
	Start      int
	End        int
	Candidates []string
	Truncated  bool
}

func Words(value string, caret int, ignoreCase bool) Result {
	offset := caretByteOffset(value, caret)
	if offset <= 0 {
		return Result{}
	}
	start, startCharacter := offset, caret
	for start > 0 {
		character, width := utf8.DecodeLastRuneInString(value[:start])
		if !wordCharacter(character) {
			break
		}
		start -= width
		startCharacter--
		if offset-start > maxWordBytes {
			return Result{}
		}
	}
	if start == offset {
		return Result{}
	}
	prefix := value[start:offset]
	endOfCurrent := offset
	for endOfCurrent < len(value) && endOfCurrent-start <= maxWordBytes {
		character, width := utf8.DecodeRuneInString(value[endOfCurrent:])
		if !wordCharacter(character) {
			break
		}
		endOfCurrent += width
	}
	current := value[start:endOfCurrent]
	result := Result{Start: startCharacter, End: caret}
	seen := make(map[string]struct{})
	for candidate := range strings.FieldsFuncSeq(value, func(character rune) bool { return !wordCharacter(character) }) {
		if len(candidate) > maxWordBytes || candidate == current {
			continue
		}
		if _, duplicate := seen[candidate]; duplicate {
			continue
		}
		if ignoreCase && !equalFoldPrefix(candidate, prefix) || !ignoreCase && !strings.HasPrefix(candidate, prefix) {
			continue
		}
		// A returned completion should not keep the full document snapshot alive.
		candidate = strings.Clone(candidate)
		seen[candidate] = struct{}{}
		result.Candidates = append(result.Candidates, candidate)
	}
	if ignoreCase {
		slices.SortFunc(result.Candidates, func(first, second string) int {
			if order := strings.Compare(strings.ToLower(first), strings.ToLower(second)); order != 0 {
				return order
			}
			return strings.Compare(first, second)
		})
	} else {
		slices.Sort(result.Candidates)
	}
	return result
}

func wordCharacter(character rune) bool {
	return character == '_' || unicode.IsLetter(character) || unicode.IsNumber(character) || unicode.IsMark(character)
}
