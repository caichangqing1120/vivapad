package completion

import (
	"slices"
	"strings"
	"unicode"
)

type Result struct {
	Start      int
	End        int
	Candidates []string
	Truncated  bool
}

func Words(value string, caret int, ignoreCase bool) Result {
	characters := []rune(value)
	if caret <= 0 || caret > len(characters) || !wordCharacter(characters[caret-1]) {
		return Result{}
	}
	start := caret - 1
	for start > 0 && wordCharacter(characters[start-1]) {
		start--
	}
	prefix := string(characters[start:caret])
	if len(prefix) >= 256 {
		return Result{}
	}
	endOfCurrent := caret
	for endOfCurrent < len(characters) && wordCharacter(characters[endOfCurrent]) {
		endOfCurrent++
	}
	current := string(characters[start:endOfCurrent])
	result := Result{Start: start, End: caret}
	seen := make(map[string]bool)
	for position := 0; position < len(characters); {
		if !wordCharacter(characters[position]) {
			position++
			continue
		}
		wordStart := position
		for position < len(characters) && wordCharacter(characters[position]) {
			position++
		}
		candidate := string(characters[wordStart:position])
		if candidate == current || len(candidate) >= 256 || seen[candidate] {
			continue
		}
		candidateRunes := []rune(candidate)
		if len(candidateRunes) < caret-start {
			continue
		}
		candidatePrefix := string(candidateRunes[:caret-start])
		if ignoreCase && !strings.EqualFold(candidatePrefix, prefix) || !ignoreCase && candidatePrefix != prefix {
			continue
		}
		seen[candidate] = true
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
