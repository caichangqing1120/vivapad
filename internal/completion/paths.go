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
	"syscall"
	"unicode"
)

const pathPunctuation = "\"'()[]<>"

func Paths(value string, caret int, base string) (Result, error) {
	characters := []rune(value)
	raw, start := pathAtCaret(characters, caret)
	if raw == "" {
		return Result{}, nil
	}
	if len(raw) > 4096 {
		return Result{}, fmt.Errorf("path prefix exceeds 4096 bytes")
	}
	if runtime.GOOS != "windows" && len(raw) >= 3 && unicode.IsLetter(rune(raw[0])) && raw[1] == ':' && (raw[2] == '\\' || raw[2] == '/') {
		return Result{}, fmt.Errorf("Windows path completion requires Windows")
	}
	end := pathTokenEnd(characters, start, caret)
	result := Result{Start: start, End: end}
	if end > caret {
		tail := string(characters[caret:end])
		if strings.ContainsRune(tail, '/') || runtime.GOOS == "windows" && strings.ContainsRune(tail, '\\') {
			return result, nil
		}
		if len(raw)+len(tail) > 4096 {
			return Result{}, fmt.Errorf("path token exceeds 4096 bytes")
		}
		full, err := resolvePath(raw+tail, base)
		if err != nil {
			return Result{}, err
		}
		if _, err := os.Stat(full); err == nil {
			return result, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return Result{}, err
		}
	}
	resolved, err := resolvePath(raw, base)
	if err != nil {
		return Result{}, err
	}
	parent, suffix, display := "", "", ""
	info, err := os.Stat(resolved)
	explicitDirectory := os.IsPathSeparator(raw[len(raw)-1])
	if explicitDirectory {
		if err != nil {
			return Result{}, err
		}
		if !info.IsDir() {
			return Result{}, fmt.Errorf("path is not a directory: %s", resolved)
		}
	}
	switch {
	case err == nil && info.Mode().IsRegular():
		if end > caret {
			result.Candidates = []string{raw}
		}
		return result, nil
	case err == nil && info.IsDir():
		parent, display = resolved, raw
		if !explicitDirectory {
			display += string(filepath.Separator)
		}
	case err == nil:
		return Result{}, fmt.Errorf("path is not a regular file or directory: %s", resolved)
	case errors.Is(err, os.ErrNotExist):
		parent, suffix = filepath.Split(resolved)
		if parent == "" {
			parent = "."
		}
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
		mode := entry.Type()
		if mode&os.ModeSymlink != 0 {
			info, err := os.Stat(joinLookupPath(parent, entry.Name()))
			if err != nil {
				continue
			}
			mode = info.Mode()
		}
		if mode.IsDir() || mode.IsRegular() {
			matches = append(matches, candidate{name: entry.Name(), directory: mode.IsDir()})
		}
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

// PathNamesFile checks the current file system without listing the directory.
func PathNamesFile(raw, base string) (bool, error) {
	info, err := pathInfo(raw, base)
	return info != nil && info.Mode().IsRegular(), err
}

// PathCandidateAvailable checks a selected suggestion without rescanning its directory.
func PathCandidateAvailable(raw, base string) (bool, error) {
	info, err := pathInfo(raw, base)
	if errors.Is(err, syscall.ENOTDIR) {
		return false, nil
	}
	if err != nil || info == nil {
		return false, err
	}
	if raw != "" && os.IsPathSeparator(raw[len(raw)-1]) {
		return info.IsDir(), nil
	}
	return info.Mode().IsRegular(), nil
}

func pathInfo(raw, base string) (os.FileInfo, error) {
	path, err := resolvePath(raw, base)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return info, err
}

func resolvePath(raw, base string) (string, error) {
	if strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return joinLookupPath(home, raw[2:]), nil
	}
	if !filepath.IsAbs(raw) {
		return joinLookupPath(base, raw), nil
	}
	return raw, nil
}

func joinLookupPath(dir, tail string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, tail)
	}
	// On Unix, a symlink must be followed before resolving a later "..".
	if dir == "" || os.IsPathSeparator(dir[len(dir)-1]) {
		return dir + tail
	}
	return dir + string(filepath.Separator) + tail
}

func pathPrefix(name, prefix string) bool {
	nameRunes, prefixRunes := []rune(name), []rune(prefix)
	return len(nameRunes) >= len(prefixRunes) && strings.EqualFold(string(nameRunes[:len(prefixRunes)]), prefix)
}

func pathAtCaret(characters []rune, caret int) (string, int) {
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
	for index := caret - 1; index >= lineStart && !strings.ContainsRune(pathPunctuation, characters[index]); index-- {
		if index+2 >= caret || !unicode.IsLetter(characters[index]) || characters[index+1] != ':' || characters[index+2] != '\\' && characters[index+2] != '/' {
			continue
		}
		if index == lineStart || barePathBoundary(characters[index-1]) {
			return string(characters[index:caret]), index
		}
	}
	start := caret
	for start > lineStart && !barePathBoundary(characters[start-1]) {
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
	_, start := pathAtCaret(characters, caret)
	if caret == 0 || start == 0 && barePathBoundary(characters[caret-1]) {
		start = caret
	}
	end := pathTokenEnd(characters, start, caret)
	if start >= end || end-start > 4096 {
		return "", 0, 0
	}
	return string(characters[start:end]), start, end
}

func pathTokenEnd(characters []rune, start, caret int) int {
	quote := rune(0)
	if start > 0 && (characters[start-1] == '"' || characters[start-1] == '\'') {
		quote = characters[start-1]
	}
	end := caret
	for end < len(characters) {
		character := characters[end]
		if character == '\r' || character == '\n' || quote != 0 && character == quote ||
			quote == 0 && barePathBoundary(character) {
			break
		}
		end++
	}
	return end
}

func barePathBoundary(character rune) bool {
	return unicode.IsSpace(character) || strings.ContainsRune(pathPunctuation, character)
}

func pathLike(raw string) bool {
	return raw != ""
}
