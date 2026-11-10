package completion

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode"
)

func Paths(value string, caret int, base string) (Result, error) {
	raw, start := pathAtCaret(value, caret)
	if raw == "" {
		return Result{}, nil
	}
	if len(raw) > 4096 {
		return Result{}, fmt.Errorf("path prefix exceeds 4096 bytes")
	}
	if runtime.GOOS != "windows" && len(raw) >= 3 && unicode.IsLetter(rune(raw[0])) && raw[1] == ':' && (raw[2] == '\\' || raw[2] == '/') {
		return Result{}, fmt.Errorf("Windows path completion requires Windows")
	}
	resolved := raw
	if strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return Result{}, err
		}
		resolved = filepath.Join(home, raw[2:])
	} else if !filepath.IsAbs(raw) {
		resolved = filepath.Join(base, raw)
	}
	result := Result{Start: start, End: caret}
	parent, suffix, display := "", "", ""
	info, err := os.Stat(resolved)
	switch {
	case err == nil && info.Mode().IsRegular():
		return result, nil
	case err == nil && info.IsDir():
		parent, display = resolved, raw
		if !strings.HasSuffix(display, string(filepath.Separator)) {
			display += string(filepath.Separator)
		}
	case err == nil:
		return Result{}, fmt.Errorf("path is not a regular file or directory: %s", resolved)
	case errors.Is(err, os.ErrNotExist):
		parent, suffix = filepath.Dir(resolved), filepath.Base(resolved)
		display = raw[:len(raw)-len(suffix)]
	default:
		return Result{}, err
	}
	directory, err := os.Open(parent)
	if err != nil {
		return Result{}, err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(2001)
	if err != nil && !errors.Is(err, io.EOF) {
		return Result{}, err
	}
	if len(entries) > 2000 {
		entries = entries[:2000]
		result.Truncated = true
	}
	type candidate struct {
		name      string
		directory bool
	}
	var matches []candidate
	for _, entry := range entries {
		if entry.Name() == "." || entry.Name() == ".." || !pathPrefix(entry.Name(), suffix) {
			continue
		}
		matches = append(matches, candidate{name: entry.Name(), directory: entry.IsDir()})
	}
	slices.SortFunc(matches, func(first, second candidate) int {
		if first.directory != second.directory {
			if first.directory {
				return -1
			}
			return 1
		}
		if order := strings.Compare(strings.ToLower(first.name), strings.ToLower(second.name)); order != 0 {
			return order
		}
		return strings.Compare(first.name, second.name)
	})
	for _, match := range matches {
		path := display + match.name
		if match.directory {
			path += string(filepath.Separator)
		}
		result.Candidates = append(result.Candidates, path)
	}
	return result, nil
}

func pathPrefix(name, prefix string) bool {
	nameRunes, prefixRunes := []rune(name), []rune(prefix)
	return len(nameRunes) >= len(prefixRunes) && strings.EqualFold(string(nameRunes[:len(prefixRunes)]), prefix)
}

func pathAtCaret(value string, caret int) (string, int) {
	characters := []rune(value)
	if caret <= 0 || caret > len(characters) {
		return "", 0
	}
	lineStart := caret
	for lineStart > 0 && characters[lineStart-1] != '\n' && characters[lineStart-1] != '\r' {
		lineStart--
	}
	quoteStart, quote := -1, rune(0)
	for index := lineStart; index < caret; index++ {
		if characters[index] != '\'' && characters[index] != '"' {
			continue
		}
		if quoteStart < 0 {
			quoteStart, quote = index, characters[index]
		} else if characters[index] == quote {
			quoteStart, quote = -1, 0
		}
	}
	if quoteStart >= 0 {
		start := quoteStart + 1
		raw := string(characters[start:caret])
		if pathLike(raw) {
			return raw, start
		}
	}
	for index := caret - 3; index >= lineStart; index-- {
		if !unicode.IsLetter(characters[index]) || characters[index+1] != ':' || characters[index+2] != '\\' && characters[index+2] != '/' {
			continue
		}
		if index == lineStart || unicode.IsSpace(characters[index-1]) || strings.ContainsRune("\"'(", characters[index-1]) {
			return string(characters[index:caret]), index
		}
	}
	start := caret
	for start > lineStart && !unicode.IsSpace(characters[start-1]) && !strings.ContainsRune("\"'()", characters[start-1]) {
		start--
	}
	raw := string(characters[start:caret])
	if pathLike(raw) {
		return raw, start
	}
	return "", 0
}

func PathTokenAtCaret(value string, caret int) (string, int, int) {
	characters := []rune(value)
	if caret < 0 || caret > len(characters) {
		return "", 0, 0
	}
	_, start := pathAtCaret(value, caret)
	if caret == 0 || start == 0 && caret > 0 && (unicode.IsSpace(characters[caret-1]) || strings.ContainsRune("\"'()[]<>", characters[caret-1])) {
		start = caret
	}
	quote := rune(0)
	if start > 0 && (characters[start-1] == '"' || characters[start-1] == '\'') {
		quote = characters[start-1]
	}
	end := caret
	for end < len(characters) {
		character := characters[end]
		if character == '\r' || character == '\n' || quote != 0 && character == quote ||
			quote == 0 && (unicode.IsSpace(character) || strings.ContainsRune("\"'()[]<>", character)) {
			break
		}
		end++
	}
	if start >= end || end-start > 4096 {
		return "", 0, 0
	}
	return string(characters[start:end]), start, end
}

func pathLike(raw string) bool {
	return raw != ""
}
