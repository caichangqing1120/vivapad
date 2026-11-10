package searchfiles

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

type Options struct {
	Pattern      string
	Recursive    bool
	MaxFileBytes int64
	MaxMatches   int
	Encoding     document.Encoding
}

type Match struct {
	Path       string
	DiskDigest string
	Encoding   document.Encoding
	Line       int
	Column     int
	Preview    string
	Text       string
	Start      int
	End        int
}

type Result struct {
	Matches       []Match
	FilesSearched int
	FilesSkipped  int
	Truncated     bool
}

var errLimit = errors.New("match limit reached")

func Find(ctx context.Context, root, query string, search document.SearchOptions, options Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if query == "" {
		return Result{}, fmt.Errorf("enter text to find")
	}
	if options.Encoding != document.UTF8 && !options.Encoding.IsLegacy() {
		return Result{}, fmt.Errorf("unsupported directory search encoding %d", options.Encoding)
	}
	if _, err := document.New("").MatchesWith(query, search); err != nil {
		return Result{}, err
	}
	patterns := strings.Split(options.Pattern, ";")
	if options.Pattern == "" {
		patterns = []string{"*"}
	}
	for index, pattern := range patterns {
		patterns[index] = strings.TrimSpace(pattern)
		if patterns[index] == "" {
			return Result{}, fmt.Errorf("empty file pattern")
		}
		if _, err := filepath.Match(patterns[index], "example.txt"); err != nil {
			return Result{}, err
		}
	}
	if options.MaxFileBytes <= 0 {
		options.MaxFileBytes = 8 << 20
	}
	if options.MaxMatches <= 0 {
		options.MaxMatches = 2000
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return Result{}, fmt.Errorf("search root is not a directory: %s", absolute)
	}
	result := Result{}
	visit := func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if path == absolute {
				return walkErr
			}
			result.FilesSkipped++
			return nil
		}
		if entry.IsDir() {
			if path != absolute && !options.Recursive {
				return filepath.SkipDir
			}
			return nil
		}
		matched := false
		for _, pattern := range patterns {
			matches, _ := filepath.Match(pattern, entry.Name())
			matched = matched || matches
		}
		if !matched {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			result.FilesSkipped++
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > options.MaxFileBytes {
			result.FilesSkipped++
			return nil
		}
		doc, err := openWithEncoding(path, options.Encoding)
		if err != nil {
			result.FilesSkipped++
			return nil
		}
		result.FilesSearched++
		spans, err := doc.MatchesWith(query, search)
		if err != nil {
			return err
		}
		return appendMatches(ctx, &result, doc, spans, options.MaxMatches)
	}
	if err := filepath.WalkDir(absolute, visit); err != nil && !errors.Is(err, errLimit) {
		return result, err
	}
	return result, nil
}

func FindDocument(doc *document.Document, query string, search document.SearchOptions, maxMatches int) (Result, error) {
	if doc == nil || query == "" {
		return Result{}, fmt.Errorf("enter text to find in an open document")
	}
	spans, err := doc.MatchesWith(query, search)
	if err != nil {
		return Result{}, err
	}
	if maxMatches <= 0 {
		maxMatches = 2000
	}
	result := Result{FilesSearched: 1}
	if err := appendMatches(context.Background(), &result, doc, spans, maxMatches); err != nil && !errors.Is(err, errLimit) {
		return Result{}, err
	}
	return result, nil
}

func appendMatches(ctx context.Context, result *Result, doc *document.Document, spans []text.Span, maxMatches int) error {
	value := doc.Editor.String()
	line, lineStart, scanned := 1, 0, 0
	for _, span := range spans {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(result.Matches) >= maxMatches {
			result.Truncated = true
			return errLimit
		}
		for scanned < span.Start {
			if value[scanned] == '\n' {
				line++
				lineStart = scanned + 1
			}
			scanned++
		}
		lineEnd := len(value)
		if newline := strings.IndexByte(value[lineStart:], '\n'); newline >= 0 {
			lineEnd = lineStart + newline
		}
		preview := previewLine(value[lineStart:lineEnd], span.Start-lineStart, min(span.End, lineEnd)-lineStart)
		result.Matches = append(result.Matches, Match{
			Path: doc.Path, DiskDigest: doc.DiskDigest(), Encoding: doc.Encoding(), Line: line, Column: utf8.RuneCountInString(value[lineStart:span.Start]) + 1,
			Preview: preview, Text: value[span.Start:span.End], Start: span.Start, End: span.End,
		})
	}
	return nil
}

func openWithEncoding(path string, encoding document.Encoding) (*document.Document, error) {
	if encoding.IsLegacy() {
		return document.OpenWithEncoding(path, encoding)
	}
	return document.Open(path)
}

func previewLine(line string, matchStart, matchEnd int) string {
	if utf8.RuneCountInString(line) <= 160 {
		return strings.TrimSpace(line)
	}
	matchStart = min(max(matchStart, 0), len(line))
	matchEnd = min(max(matchEnd, matchStart), len(line))
	first := max(0, utf8.RuneCountInString(line[:matchStart])-60)
	if end := utf8.RuneCountInString(line[:matchEnd]); first+160 < end {
		first = end - 160
	}
	startByte, endByte, index := 0, len(line), 0
	for offset := range line {
		if index == first {
			startByte = offset
		}
		if index == first+160 {
			endByte = offset
			break
		}
		index++
	}
	preview := strings.TrimSpace(line[startByte:endByte])
	if startByte > 0 {
		preview = "..." + preview
	}
	if endByte < len(line) {
		preview += "..."
	}
	return preview
}
