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
	"vivapad/internal/changehistory"
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
	eolEditor     *text.Editor
	eolRevision   uint64
	eolCacheValid bool
	eolCacheEOL   string
	eolUniform    bool
	eolLabels     []byte
	eolCache      []string
	uniformEOL    bool

	// history is the change-history consumer bound to historyEditor when the
	// document was created. It is never rebound in place: a replaced editor
	// retires the source and reads as an explicit unavailable history.
	history       *changehistory.Consumer
	historyEditor *text.Editor
	historyErr    error
	historyBound  bool
}

type LineEnding uint8

const (
	LF LineEnding = iota
	CRLF
	CR
)

func New(name string) *Document {
	doc := newDocument(name, text.NewEditor(""))
	doc.bindChangeHistory()
	return doc
}

// newDocument prepares a document around editor without enabling the history
// producer yet. Callers that own a final editor enable it with
// bindChangeHistory once the editor can no longer be replaced.
func newDocument(name string, editor *text.Editor) *Document {
	doc := &Document{Name: name, Editor: editor, newline: "\n", savedNewline: "\n"}
	labels := bytes.Repeat([]byte{'L'}, strings.Count(editor.String(), "\n"))
	if editor.TrackLFLabels(labels, 'L', mapLFLineEndings) {
		doc.eolEditor = editor
	}
	editor.SetSavePoint()
	editor.TrackLineEdits()
	return doc
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
		return nil, &DecodeError{Path: absolute, Err: err}
	}
	value, newline, breaks := normalizeLineEndings(value)
	// Build the final editor first: there is no empty editor to replace and
	// therefore no chance that a throwaway editor becomes the history source.
	doc := newDocument(filepath.Base(absolute), text.NewEditor(value))
	doc.Path, doc.newline, doc.encoding = absolute, newline, encoding
	labels := bytes.Repeat([]byte{lineEndingCode(newline)}, strings.Count(value, "\n"))
	for index, ending := range breaks {
		labels[index] = lineEndingCode(ending)
	}
	if !doc.Editor.SetLFLabels(labels, lineEndingCode(newline)) {
		return nil, fmt.Errorf("%s: cannot initialize EOL labels", absolute)
	}
	doc.Editor.MoveTo(0, false)
	doc.Editor.SetSavePoint()
	doc.Editor.TrackLineEdits()
	doc.saved = value
	doc.savedDisk = original
	doc.savedDigest = digest(original)
	doc.savedNewline, doc.savedEncoding = newline, encoding
	// Bind only after the editor is final and before the first undoable edit.
	doc.bindChangeHistory()
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
	newline := "\n"
	switch ending {
	case CRLF:
		newline = "\r\n"
	case CR:
		newline = "\r"
	}
	labels, err := doc.currentLFLabels()
	if err != nil || newline == doc.newline && uniformLFLabels(labels, lineEndingCode(newline)) {
		return
	}
	if !doc.Editor.UnifyLFLabels(lineEndingCode(newline)) {
		return
	}
	doc.uniformEOL = doc.uniformEOL || !uniformLFLabels(labels, lineEndingCode(doc.newline))
	doc.newline = newline
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
	if doc.Path != "" && absolute != doc.Path {
		requested, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if requested == doc.Path {
			return fmt.Errorf("%s no longer resolves to its original path; use Save As", doc.Name)
		}
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
	data, err := doc.encodeForSave(value)
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
	doc.uniformEOL = false
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
	data, err := doc.encodeForSave(doc.Editor.String())
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
	_, err := doc.encodeForSave(doc.Editor.String())
	return err
}

// SaveDiffersFromSnapshot compares save output with the captured disk bytes
// without reading or writing the current file.
func (doc *Document) SaveDiffersFromSnapshot() (bool, error) {
	data, err := doc.encodeForSave(doc.Editor.String())
	if err != nil {
		return false, err
	}
	return !bytes.Equal(data, doc.savedDisk), nil
}

func (doc *Document) encodeForSave(value string) ([]byte, error) {
	physical, err := doc.physicalValue(value)
	if err != nil {
		return nil, err
	}
	return encodeText(physical, doc.encoding)
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
