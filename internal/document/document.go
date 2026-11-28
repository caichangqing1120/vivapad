package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"vivapad/internal/changehistory"
	"vivapad/internal/largefile"
)

type Document struct {
	Path        string
	Name        string
	Editor      *text.Editor
	RTL         bool // View state shared by clones of this document.
	lightweight bool
	large       *largefile.File

	saved         string
	savedDisk     []byte
	savedDigest   string
	savedNewline  string
	savedEncoding Encoding
	savedEOL      string
	formatHistory *formatHistory
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
	return open(path, nil, 0)
}

func OpenWithEncoding(path string, encoding Encoding) (*Document, error) {
	if legacyCodec(encoding) == nil {
		return nil, fmt.Errorf("unsupported explicit encoding %d", encoding)
	}
	return open(path, &encoding, 0)
}

func open(path string, selected *Encoding, maxBytes int64) (*Document, error) {
	absolute, err := canonicalOpenPath(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", absolute)
	}
	interactive := maxBytes == 0
	if interactive {
		if info.Size() > LargeFileThreshold {
			return openLarge(absolute, selected)
		}
		maxBytes = LargeFileThreshold
	}
	file, err := readDecodedAt(absolute, selected, maxBytes)
	if errors.Is(err, ErrSearchFileTooLarge) && interactive {
		return openLarge(absolute, selected)
	}
	if err != nil {
		return nil, err
	}
	value, newline, breaks := normalizeLineEndings(file.value)
	// Build the final editor first: there is no empty editor to replace and
	// therefore no chance that a throwaway editor becomes the history source.
	doc := newDocument(filepath.Base(file.path), text.NewEditor(value))
	doc.lightweight = int64(len(file.data)) >= LightweightThreshold
	doc.Path, doc.newline, doc.encoding = file.path, newline, file.encoding
	labels := bytes.Repeat([]byte{lineEndingCode(newline)}, strings.Count(value, "\n"))
	for index, ending := range breaks {
		labels[index] = lineEndingCode(ending)
	}
	if !doc.Editor.SetLFLabels(labels, lineEndingCode(newline)) {
		return nil, fmt.Errorf("%s: cannot initialize EOL labels", file.path)
	}
	if len(breaks) != 0 {
		doc.savedEOL = string(labels)
	}
	doc.Editor.MoveTo(0, false)
	doc.Editor.SetSavePoint()
	doc.Editor.TrackLineEdits()
	doc.saved = value
	doc.savedDisk = file.data
	doc.savedDigest = digest(file.data)
	doc.savedNewline, doc.savedEncoding = newline, file.encoding
	// Bind only after the editor is final and before the first undoable edit.
	if !doc.lightweight {
		doc.bindChangeHistory()
	} else {
		doc.Editor.TrackHistory()
	}
	return doc, nil
}

type decodedFile struct {
	path, value string
	data        []byte
	encoding    Encoding
}

func readDecodedFile(path string, selected *Encoding, maxBytes int64) (decodedFile, error) {
	absolute, err := canonicalOpenPath(path)
	if err != nil {
		return decodedFile{}, err
	}
	return readDecodedAt(absolute, selected, maxBytes)
}

func canonicalOpenPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

func readDecodedAt(absolute string, selected *Encoding, maxBytes int64) (decodedFile, error) {
	data, err := readDocumentFile(absolute, maxBytes)
	if err != nil {
		return decodedFile{}, err
	}
	var value string
	var encoding Encoding
	if selected == nil {
		value, encoding, err = decodeTextWithNFOFallback(data, strings.EqualFold(filepath.Ext(absolute), ".nfo"))
	} else {
		value, encoding, err = decodeLegacy(data, *selected)
	}
	if err != nil {
		return decodedFile{}, &DecodeError{Path: absolute, Err: err}
	}
	return decodedFile{path: absolute, value: value, data: data, encoding: encoding}, nil
}

func (doc *Document) Dirty() bool {
	if doc.large != nil {
		return false
	}
	history := doc.syncFormatHistory()
	return doc.recovered || doc.uniformEOL || !doc.Editor.AtSavePoint() || doc.Editor.String() != doc.saved || doc.newline != doc.savedNewline || doc.encoding != doc.savedEncoding || history != nil && doc.EOLSequence() != doc.savedEOL
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
	if doc.large != nil || ending != LF && ending != CRLF && ending != CR {
		return
	}
	newline := newlineFor(ending)
	labels, err := doc.currentLFLabels()
	if err != nil || newline == doc.newline && uniformLFLabels(labels, lineEndingCode(newline)) {
		return
	}
	if !doc.Editor.UnifyLFLabels(lineEndingCode(newline)) {
		return
	}
	doc.uniformEOL = doc.uniformEOL || !uniformLFLabels(labels, lineEndingCode(doc.newline))
	doc.newline = newline
	doc.clearFormatUndo()
}

func newlineFor(ending LineEnding) string {
	switch ending {
	case CRLF:
		return "\r\n"
	case CR:
		return "\r"
	default:
		return "\n"
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
	if doc.large != nil {
		return ErrLargeFileReadOnly
	}
	if path == "" {
		path = doc.Path
	}
	absolute, err := CanonicalSavePath(path)
	if err != nil {
		return err
	}
	sameSource := doc.Path != "" && absolute == doc.Path
	if doc.Path != "" && absolute != doc.Path {
		requested, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if requested == doc.Path {
			return fmt.Errorf("%s no longer resolves to its original path; use Save As", doc.Name)
		}
		source, sourceErr := os.Stat(doc.Path)
		target, targetErr := os.Stat(absolute)
		sameSource = sourceErr == nil && targetErr == nil && os.SameFile(source, target)
	}
	if sameSource {
		matches, err := diskBytesMatch(absolute, doc.savedDisk)
		if err != nil {
			return err
		}
		if !matches {
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
	doc.savedEOL = doc.EOLSequence()
	doc.uniformEOL = false
	doc.recovered = false
	doc.Editor.SetSavePoint()
	return nil
}

var ErrDiskPathRedirected = errors.New("file path no longer resolves to its original location")

// CheckDiskPath verifies that the document still names a regular file at its bound path.
func (doc *Document) CheckDiskPath() error {
	return CheckRecordedPath(doc.Path)
}

// CheckRecordedPath verifies that a previously canonicalized path still names a regular file there.
func CheckRecordedPath(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if resolved != path {
		return fmt.Errorf("%w: %s", ErrDiskPathRedirected, path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}

// DiskMatchesSaved compares the current file with the saved bytes without
// decoding it or allocating another copy of the file.
func (doc *Document) DiskMatchesSaved() (bool, error) {
	if doc.large != nil {
		return false, ErrLargeFileReadOnly
	}
	return diskBytesMatch(doc.Path, doc.savedDisk)
}

func diskBytesMatch(path string, expected []byte) (bool, error) {
	file, err := openDocumentFile(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	return fileBytesMatch(file, info.Size(), expected)
}

func fileBytesMatch(file io.Reader, size int64, expected []byte) (bool, error) {
	if size != int64(len(expected)) {
		return false, nil
	}
	var buffer [32 << 10]byte
	for len(expected) != 0 {
		chunk := buffer[:min(len(expected), len(buffer))]
		if _, err := io.ReadFull(file, chunk); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return false, nil
			}
			return false, err
		}
		if !bytes.Equal(chunk, expected[:len(chunk)]) {
			return false, nil
		}
		expected = expected[len(chunk):]
	}
	// Check for growth after Stat as well as changes within the saved length.
	_, err := io.ReadFull(file, buffer[:1])
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func (doc *Document) SaveCopy(path string) error {
	if doc.large != nil {
		return ErrLargeFileReadOnly
	}
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
	value := doc.Editor.String()
	prefix := value[:doc.Editor.ByteOffset(doc.Editor.Caret())]
	lineStart := strings.LastIndexByte(prefix, '\n') + 1
	return strings.Count(prefix, "\n") + 1, utf8.RuneCountInString(prefix[lineStart:]) + 1
}

func (doc *Document) LineCount() int {
	if doc.large != nil {
		return doc.large.Progress().Lines
	}
	return strings.Count(doc.Editor.String(), "\n") + 1
}
