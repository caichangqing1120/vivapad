package batchconvert

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"vivapad/internal/document"
)

type EOL uint8

const (
	Keep EOL = iota
	LF
	CRLF
	CR
	Mixed
	None
)

func (ending EOL) String() string {
	switch ending {
	case LF:
		return "LF"
	case CRLF:
		return "CRLF"
	case CR:
		return "CR"
	case Mixed:
		return "Mixed"
	case None:
		return "None"
	default:
		return "Keep unchanged"
	}
}

type Options struct {
	Root           string
	Glob           string
	Recursive      bool
	MaxDepth       int
	MaxFileSize    int64
	MaxFiles       int
	TargetEncoding *document.Encoding
	TargetEOL      EOL
}

type Action string

const (
	Convert   Action = "convert"
	Unchanged Action = "unchanged"
	Skip      Action = "skip"
)

const (
	ReasonVCS      = "version-control directory"
	ReasonSymlink  = "symbolic link"
	ReasonSpecial  = "not a regular file"
	ReasonTooLarge = "file exceeds size limit"
	ReasonBinary   = "binary or unsupported text encoding"
	ReasonReadOnly = "read-only file"
)

type Entry struct {
	Path, Relative                 string
	SourceEncoding, TargetEncoding document.Encoding
	SourceEOL, TargetEOL           EOL
	HasFormat                      bool
	Action                         Action
	Reason                         string
}

type plannedFile struct {
	entry    Entry
	revision revision
}

type Preview struct {
	root      string
	rootInfo  os.FileInfo
	limit     int64
	files     []plannedFile
	truncated bool
}

func (preview *Preview) Entries() []Entry {
	entries := make([]Entry, len(preview.files))
	for index, file := range preview.files {
		entries[index] = file.entry
	}
	return entries
}

func (preview *Preview) Truncated() bool { return preview.truncated }

type FileError struct {
	Path, Relative string
	Err            error
}

type Result struct {
	Converted, Unchanged, Skipped, Failed int
	Errors                                []FileError
	Err                                   error
}

func Plan(ctx context.Context, opts Options) (*Preview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.Root) == "" || opts.MaxFileSize < 0 || opts.MaxDepth < 0 || opts.MaxFiles < 0 || opts.TargetEOL > CR ||
		opts.TargetEncoding != nil && *opts.TargetEncoding > document.GB18030 {
		return nil, errors.New("invalid batch conversion options")
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 32
	}
	if opts.MaxFileSize == 0 {
		opts.MaxFileSize = 16 << 20
	}
	if opts.MaxFiles == 0 {
		opts.MaxFiles = 5000
	}
	patterns, err := parseGlobs(opts.Glob)
	if err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || insideVCS(absolute) {
		return nil, errors.New("select a real directory outside version-control metadata")
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	if insideVCS(absolute) {
		return nil, errors.New("select a directory outside version-control metadata")
	}
	preview := &Preview{root: absolute, rootInfo: info, limit: opts.MaxFileSize}
	root, err := preview.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	err = fs.WalkDir(root.FS(), ".", func(name string, item fs.DirEntry, scanErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return scanErr
		}
		entry := Entry{Path: filepath.Join(absolute, filepath.FromSlash(name)), Relative: filepath.FromSlash(name), Action: Skip}
		var fileRevision revision
		switch {
		case isVCS(item.Name()):
			entry.Reason = ReasonVCS
		case scanErr != nil:
			entry.Reason = scanErr.Error()
		case item.IsDir():
			if !opts.Recursive || strings.Count(name, "/") >= opts.MaxDepth {
				return fs.SkipDir
			}
			return nil
		case !matchesGlob(patterns, name):
			return nil
		}
		if len(preview.files) >= opts.MaxFiles {
			preview.truncated = true
			return fs.SkipAll
		}
		if entry.Reason == "" {
			switch {
			case item.Type()&os.ModeSymlink != 0:
				entry.Reason = ReasonSymlink
			case !item.Type().IsRegular():
				entry.Reason = ReasonSpecial
			default:
				fileRevision = inspect(root, &entry, opts)
			}
		}
		preview.files = append(preview.files, plannedFile{entry: entry, revision: fileRevision})
		if item.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return preview, nil
}

func isVCS(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".hg", ".svn", ".bzr", "_darcs", ".jj":
		return true
	default:
		return false
	}
}

func insideVCS(name string) bool {
	for part := range strings.SplitSeq(name, string(filepath.Separator)) {
		if isVCS(part) {
			return true
		}
	}
	return false
}

func parseGlobs(value string) ([]string, error) {
	var patterns []string
	for _, pattern := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' }) {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			if _, err := path.Match(pattern, ""); err != nil {
				return nil, fmt.Errorf("invalid glob %q: %w", pattern, err)
			}
			patterns = append(patterns, pattern)
		}
	}
	return patterns, nil
}

func matchesGlob(patterns []string, name string) bool {
	return len(patterns) == 0 || slices.ContainsFunc(patterns, func(pattern string) bool {
		candidate := name
		if !strings.Contains(pattern, "/") {
			candidate = path.Base(name)
		}
		match, _ := path.Match(pattern, candidate)
		return match
	})
}

func inspect(root *os.Root, entry *Entry, opts Options) revision {
	if err := checkParents(root, filepath.Dir(entry.Relative)); err != nil {
		entry.Reason = err.Error()
		return revision{}
	}
	data, current, err := readSnapshot(root, entry.Relative, opts.MaxFileSize)
	if err != nil {
		entry.Reason = err.Error()
		return revision{}
	}
	value, source, err := document.DetectTextEncoding(data)
	if err != nil || strings.ContainsFunc(value, func(character rune) bool {
		return unicode.IsControl(character) && !strings.ContainsRune("\t\n\r\f", character)
	}) {
		entry.Reason = ReasonBinary
		return revision{}
	}
	entry.HasFormat = true
	entry.SourceEncoding, entry.TargetEncoding = source, source
	entry.SourceEOL = detectEOL(value)
	entry.TargetEOL = entry.SourceEOL
	if opts.TargetEncoding != nil {
		entry.TargetEncoding = *opts.TargetEncoding
	}
	if opts.TargetEOL != Keep && entry.SourceEOL != None {
		entry.TargetEOL = opts.TargetEOL
	}
	if entry.SourceEncoding == entry.TargetEncoding && entry.SourceEOL == entry.TargetEOL {
		entry.Action = Unchanged
		return revision{}
	}
	output, err := convert(value, *entry)
	if err != nil {
		entry.Reason = err.Error()
		return revision{}
	}
	if bytes.Equal(output, data) {
		entry.Action = Unchanged
		return revision{}
	}
	if current.info.Mode().Perm()&0222 == 0 {
		entry.Reason = ReasonReadOnly
		return revision{}
	}
	entry.Action = Convert
	return current
}

func detectEOL(value string) EOL {
	ending := None
	for index := strings.IndexAny(value, "\r\n"); index >= 0; index = strings.IndexAny(value, "\r\n") {
		next := LF
		if value[index] == '\r' {
			next = CR
			if index+1 < len(value) && value[index+1] == '\n' {
				next = CRLF
				index++
			}
		}
		if ending != None && ending != next {
			return Mixed
		}
		ending, value = next, value[index+1:]
	}
	return ending
}

func convert(value string, entry Entry) ([]byte, error) {
	if entry.SourceEOL != entry.TargetEOL {
		newline := map[EOL]string{LF: "\n", CRLF: "\r\n", CR: "\r"}[entry.TargetEOL]
		value = strings.NewReplacer("\r\n", newline, "\r", newline, "\n", newline).Replace(value)
	}
	return document.EncodeText(value, entry.TargetEncoding)
}
