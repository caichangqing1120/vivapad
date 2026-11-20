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
	"unicode/utf8"
)

const pathPunctuation = "\"'()[]<>"

func Paths(value string, caret int, base string) (Result, error) {
	offset := pathCaretByteOffset(value, caret)
	if offset <= 0 {
		return Result{}, nil
	}
	raw, start := pathAtCaret(value, offset)
	if raw == "" {
		return Result{}, nil
	}
	if len(raw) > 4096 {
		return Result{}, fmt.Errorf("path prefix exceeds 4096 bytes")
	}
	if runtime.GOOS != "windows" && len(raw) >= 3 && unicode.IsLetter(rune(raw[0])) && raw[1] == ':' && (raw[2] == '\\' || raw[2] == '/') {
		return Result{}, fmt.Errorf("Windows path completion requires Windows")
	}
	end := pathTokenEnd(value, start, offset)
	tail := value[offset:end]
	result := Result{Start: caret - utf8.RuneCountInString(raw), End: caret + utf8.RuneCountInString(tail)}
	if tail != "" {
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
		if tail != "" {
			result.Candidates = []string{strings.Clone(raw)}
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
		folded    string
		directory bool
	}
	var matches []candidate
	quote := pathTokenQuote(value, start)
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." || !pathPrefix(name, suffix) || !pathNameFitsToken(name, quote) {
			continue
		}
		mode := entry.Type()
		if mode&os.ModeSymlink != 0 {
			info, err := os.Stat(joinLookupPath(parent, name))
			if err != nil {
				continue
			}
			mode = info.Mode()
		}
		if mode.IsDir() || mode.IsRegular() {
			matches = append(matches, candidate{name: name, folded: strings.ToLower(name), directory: mode.IsDir()})
		}
	}
	slices.SortFunc(matches, func(first, second candidate) int {
		if first.directory != second.directory {
			if first.directory {
				return -1
			}
			return 1
		}
		if order := strings.Compare(first.folded, second.folded); order != 0 {
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
	if strings.HasPrefix(name, prefix) {
		return true
	}
	// Case folding can match runes with different UTF-8 widths, such as K and Kelvin sign.
	end := 0
	for range prefix {
		if end == len(name) {
			return false
		}
		_, width := utf8.DecodeRuneInString(name[end:])
		end += width
	}
	return strings.EqualFold(name[:end], prefix)
}

func pathCaretByteOffset(value string, caret int) int {
	if caret < 0 || caret > len(value) {
		return -1
	}
	for offset := range value {
		if caret == 0 {
			return offset
		}
		caret--
	}
	if caret == 0 {
		return len(value)
	}
	return -1
}

// Path token scanning uses byte offsets so a filename on a long log line
// does not require a second copy of the line.
func pathAtCaret(value string, caret int) (string, int) {
	if caret <= 0 {
		return "", 0
	}
	lineStart := strings.LastIndexAny(value[:caret], "\r\n") + 1
	quoteStart, quote := -1, byte(0)
	for from := lineStart; from < caret; {
		next := strings.IndexAny(value[from:caret], "\"'")
		if next < 0 {
			break
		}
		index := from + next
		from = index + 1
		if quoteStart < 0 {
			// An apostrophe inside a word in surrounding prose cannot open a path.
			if value[index] == '\'' && index > lineStart && from < caret {
				before, _ := utf8.DecodeLastRuneInString(value[lineStart:index])
				after, _ := utf8.DecodeRuneInString(value[from:caret])
				if unicode.In(before, unicode.L, unicode.M, unicode.N) && unicode.In(after, unicode.L, unicode.M, unicode.N) {
					continue
				}
			}
			quoteStart, quote = index, value[index]
		} else if value[index] == quote {
			quoteStart, quote = -1, 0
		}
	}
	if quoteStart >= 0 && quoteStart+1 < caret {
		return value[quoteStart+1 : caret], quoteStart + 1
	}
	for end := caret; end > lineStart; {
		character, width := utf8.DecodeLastRuneInString(value[lineStart:end])
		index := end - width
		if strings.ContainsRune(pathPunctuation, character) {
			break
		}
		delimiter := end
		end = index
		if delimiter+1 >= caret || value[delimiter] != ':' || value[delimiter+1] != '\\' && value[delimiter+1] != '/' || !unicode.IsLetter(character) {
			continue
		}
		before, _ := utf8.DecodeLastRuneInString(value[lineStart:index])
		if index == lineStart || barePathBoundary(before) {
			return value[index:caret], index
		}
	}
	start := caret
	for start > lineStart {
		character, width := utf8.DecodeLastRuneInString(value[lineStart:start])
		if barePathBoundary(character) {
			break
		}
		start -= width
	}
	if start < caret {
		return value[start:caret], start
	}
	return "", 0
}

func PathTokenAtCaret(value string, caret int) (string, int, int) {
	offset := pathCaretByteOffset(value, caret)
	if offset < 0 {
		return "", 0, 0
	}
	_, start := pathAtCaret(value, offset)
	before, _ := utf8.DecodeLastRuneInString(value[:offset])
	if offset == 0 || start == 0 && barePathBoundary(before) {
		start = offset
	}
	end := pathTokenEnd(value, start, offset)
	length := utf8.RuneCountInString(value[start:end])
	if length == 0 || length > 4096 {
		return "", 0, 0
	}
	startRune := caret - utf8.RuneCountInString(value[start:offset])
	return strings.Clone(value[start:end]), startRune, startRune + length
}

func pathTokenEnd(value string, start, caret int) int {
	quote := pathTokenQuote(value, start)
	for offset, character := range value[caret:] {
		if pathTokenBoundary(character, quote) {
			return caret + offset
		}
	}
	return len(value)
}

func pathTokenQuote(value string, start int) rune {
	if start > 0 && (value[start-1] == '"' || value[start-1] == '\'') {
		return rune(value[start-1])
	}
	return 0
}

func pathNameFitsToken(name string, quote rune) bool {
	return utf8.ValidString(name) && !strings.ContainsFunc(name, func(character rune) bool {
		return pathTokenBoundary(character, quote)
	})
}

func pathTokenBoundary(character, quote rune) bool {
	return character == '\r' || character == '\n' || quote != 0 && character == quote ||
		quote == 0 && barePathBoundary(character)
}

func barePathBoundary(character rune) bool {
	return unicode.IsSpace(character) || strings.ContainsRune(pathPunctuation, character)
}
