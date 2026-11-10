package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/text"
)

type Document struct {
	Path   string
	Name   string
	Editor *text.Editor

	saved         string
	savedDisk     []byte
	savedDigest   string
	savedNewline  string
	savedEncoding Encoding
	newline       string
	encoding      Encoding
	recovered     bool
	eolBaseText   string
	eolBreaks     []string
	eolCacheText  string
	eolCache      []string
	uniformEOL    bool
}

type LineEnding uint8

const (
	LF LineEnding = iota
	CRLF
	CR
)

func New(name string) *Document {
	editor := text.NewEditor("")
	editor.SetSavePoint()
	editor.TrackLineEdits()
	return &Document{Name: name, Editor: editor, newline: "\n", savedNewline: "\n"}
}

func Open(path string) (*Document, error) {
	return open(path, nil)
}

func OpenWithEncoding(path string, encoding Encoding) (*Document, error) {
	if legacyCodec(encoding) == nil {
		return nil, fmt.Errorf("unsupported explicit encoding %d", encoding)
	}
	return open(path, &encoding)
}

func open(path string, selected *Encoding) (*Document, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	} else {
		return nil, err
	}
	data, err := os.ReadFile(absolute)
	if err != nil {
		return nil, err
	}
	original := data
	var value string
	var encoding Encoding
	if selected == nil {
		value, encoding, err = decodeText(data)
	} else {
		value, encoding, err = decodeLegacy(data, *selected)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", absolute, err)
	}
	value, newline, breaks := normalizeLineEndings(value)
	doc := New(filepath.Base(absolute))
	doc.Path, doc.newline, doc.encoding = absolute, newline, encoding
	doc.eolBaseText, doc.eolBreaks = value, breaks
	doc.Editor = text.NewEditor(value)
	doc.Editor.MoveTo(0, false)
	doc.Editor.SetSavePoint()
	doc.Editor.TrackLineEdits()
	doc.saved = value
	doc.savedDisk = original
	doc.savedDigest = digest(original)
	doc.savedNewline, doc.savedEncoding = newline, encoding
	return doc, nil
}

func (doc *Document) Dirty() bool {
	return doc.recovered || doc.uniformEOL || !doc.Editor.AtSavePoint() || doc.Editor.String() != doc.saved || doc.newline != doc.savedNewline || doc.encoding != doc.savedEncoding
}

func (doc *Document) MarkRecovered() { doc.recovered = true }

func (doc *Document) DiskDigest() string {
	return doc.savedDigest
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (doc *Document) SetLineEnding(ending LineEnding) {
	if ending != LF && ending != CRLF && ending != CR {
		return
	}
	if len(doc.eolBreaks) != 0 {
		doc.uniformEOL = true
	}
	switch ending {
	case LF:
		doc.newline = "\n"
	case CRLF:
		doc.newline = "\r\n"
	case CR:
		doc.newline = "\r"
	}
}

func (doc *Document) LineEnding() LineEnding {
	if doc.newline == "\r\n" {
		return CRLF
	}
	if doc.newline == "\r" {
		return CR
	}
	return LF
}

func (doc *Document) SetUTF8BOM(enabled bool) {
	if enabled {
		doc.SetEncoding(UTF8BOM)
	} else {
		doc.SetEncoding(UTF8)
	}
}

func (doc *Document) UTF8BOM() bool { return doc.encoding == UTF8BOM }

func (doc *Document) NewlineLabel() string {
	if doc.newline == "\r\n" {
		return "Windows (CR LF)"
	}
	if doc.newline == "\r" {
		return "Macintosh (CR)"
	}
	return "Unix (LF)"
}

func (doc *Document) Save(path string) error {
	if path == "" {
		path = doc.Path
	}
	absolute, err := CanonicalSavePath(path)
	if err != nil {
		return err
	}
	if doc.Path != "" && absolute == doc.Path {
		current, err := os.ReadFile(absolute)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, doc.savedDisk) {
			return fmt.Errorf("%s changed on disk; use Save As", doc.Name)
		}
	}
	value := doc.Editor.String()
	physical, breaks := doc.physicalValue(value)
	data, err := encodeText(physical, doc.encoding)
	if err != nil {
		return err
	}
	if err := writeAtomic(absolute, data); err != nil {
		return err
	}
	doc.Path, doc.Name, doc.saved = absolute, filepath.Base(absolute), value
	doc.savedDisk = data
	doc.savedDigest = digest(data)
	doc.savedNewline, doc.savedEncoding = doc.newline, doc.encoding
	doc.eolBaseText, doc.eolBreaks, doc.uniformEOL = value, breaks, false
	doc.eolCacheText, doc.eolCache = "", nil
	doc.recovered = false
	doc.Editor.SetSavePoint()
	return nil
}

func (doc *Document) SaveCopy(path string) error {
	absolute, err := CanonicalSavePath(path)
	if err != nil {
		return err
	}
	if doc.Path != "" && absolute == doc.Path {
		return fmt.Errorf("copy destination is the active document; use Save")
	}
	physical, _ := doc.physicalValue(doc.Editor.String())
	data, err := encodeText(physical, doc.encoding)
	if err != nil {
		return err
	}
	return writeAtomic(absolute, data)
}

func CanonicalSavePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("choose a path before saving")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(absolute); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("%s is not a regular file", absolute)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	} else if errors.Is(err, os.ErrNotExist) {
		parent, parentErr := filepath.EvalSymlinks(filepath.Dir(absolute))
		if parentErr != nil {
			return "", parentErr
		}
		absolute = filepath.Join(parent, filepath.Base(absolute))
	} else {
		return "", err
	}
	return absolute, nil
}

func (doc *Document) ValidateEncoding() error {
	physical, _ := doc.physicalValue(doc.Editor.String())
	_, err := encodeText(physical, doc.encoding)
	return err
}

func writeAtomic(path string, data []byte) (err error) {
	mode := os.FileMode(0644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
		if mode&0222 == 0 {
			return fmt.Errorf("%s: %w", path, os.ErrPermission)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".vivapad-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func (doc *Document) Find(query string, backward bool) bool {
	found, _ := doc.FindWith(query, SearchOptions{}, backward)
	return found
}

func (doc *Document) ReplaceCurrent(query, replacement string) bool {
	replaced, _ := doc.ReplaceCurrentWith(query, replacement, SearchOptions{})
	return replaced
}

func (doc *Document) ReplaceAll(query, replacement string) int {
	count, _ := doc.ReplaceAllWith(query, replacement, SearchOptions{})
	return count
}

func (doc *Document) Position() (line, column int) {
	line, column = 1, 1
	caret := doc.Editor.Caret()
	index := 0
	for _, character := range doc.Editor.String() {
		if index == caret {
			break
		}
		if character == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
		index++
	}
	return line, column
}

func (doc *Document) LineCount() int { return strings.Count(doc.Editor.String(), "\n") + 1 }
