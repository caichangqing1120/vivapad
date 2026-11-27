package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"

	"vivapad/internal/document"
)

var ErrDirty = errors.New("document has unsaved changes")

type Workspace struct {
	Documents []*document.Document
	Active    int
	nextName  int
}

func New() *Workspace {
	space := &Workspace{}
	space.NewDocument()
	return space
}

func (space *Workspace) Current() *document.Document {
	return space.Documents[space.Active]
}

func (space *Workspace) NewDocument() *document.Document {
	space.nextName++
	doc := document.New(fmt.Sprintf("new %d", space.nextName))
	space.Documents = append(space.Documents, doc)
	space.Active = len(space.Documents) - 1
	return doc
}

func (space *Workspace) AddRecovered(doc *document.Document) {
	if doc == nil {
		return
	}
	if number, err := strconv.Atoi(strings.TrimPrefix(doc.Name, "new ")); err == nil && strings.HasPrefix(doc.Name, "new ") {
		space.nextName = max(space.nextName, number)
	}
	if len(space.Documents) == 1 && space.Current().Path == "" && !space.Current().Dirty() {
		space.Documents[0] = doc
		return
	}
	space.Documents = append(space.Documents, doc)
	space.Active = len(space.Documents) - 1
}

func (space *Workspace) Select(index int) bool {
	if index < 0 || index >= len(space.Documents) {
		return false
	}
	space.Active = index
	return true
}

func (space *Workspace) Open(path string) error {
	return space.open(path, nil, openExplicit)
}

func (space *Workspace) OpenWithEncoding(path string, encoding document.Encoding) error {
	return space.open(path, &encoding, openExplicit)
}

// OpenInteractive reuses open tabs for verified spelling aliases of a user-selected path.
func (space *Workspace) OpenInteractive(path string) error {
	return space.open(path, nil, openInteractive)
}

func (space *Workspace) OpenInteractiveWithEncoding(path string, encoding document.Encoding) error {
	return space.open(path, &encoding, openInteractive)
}

// OpenRecorded opens an entry whose path was canonicalized when recorded.
// UTF8 is the history default and requests normal encoding detection.
func (space *Workspace) OpenRecorded(path string, encoding document.Encoding) error {
	if encoding == document.UTF8 {
		return space.open(path, nil, openRecorded)
	}
	return space.open(path, &encoding, openRecorded)
}

type openMode uint8

const (
	openExplicit openMode = iota
	openRecorded
	openInteractive
)

func (space *Workspace) open(path string, encoding *document.Encoding, mode openMode) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if mode == openRecorded {
		if err := document.CheckRecordedPath(path); err != nil {
			return err
		}
	} else if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	}
	openIndex := -1
	for index, doc := range space.Documents {
		if doc.Path == absolute {
			openIndex = index
			break
		}
	}
	if openIndex < 0 && mode == openInteractive {
		openIndex = space.findOpenSpellingAlias(absolute)
	}
	if openIndex >= 0 {
		doc := space.Documents[openIndex]
		if encoding != nil {
			if mode == openRecorded && doc.SavedEncoding() != *encoding {
				return fmt.Errorf("%s is already open with another saved encoding", doc.Name)
			}
			if mode != openRecorded && doc.Encoding() != *encoding {
				return fmt.Errorf("%s is already open as %s", doc.Name, doc.EncodingLabel())
			}
		}
		space.Active = openIndex
		return nil
	}
	var doc *document.Document
	if encoding == nil {
		doc, err = document.Open(absolute)
	} else {
		doc, err = document.OpenWithEncoding(absolute, *encoding)
	}
	if err != nil {
		return err
	}
	if mode == openRecorded && doc.Path != path {
		_ = doc.Close()
		return document.ErrDiskPathRedirected
	}
	return space.AddOpened(doc)
}

func (space *Workspace) findOpenSpellingAlias(path string) int {
	spelling := norm.NFC.String(path)
	var incoming os.FileInfo
	for index, doc := range space.Documents {
		if doc.Path == "" || !strings.EqualFold(spelling, norm.NFC.String(doc.Path)) {
			continue
		}
		if incoming == nil {
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				return -1
			}
			incoming = info
		}
		// A spelling match is only a hint: distinct files and redirected tabs
		// must keep their own bindings.
		if doc.CheckDiskPath() == nil {
			if existing, err := os.Lstat(doc.Path); err == nil && os.SameFile(existing, incoming) {
				return index
			}
		}
	}
	return -1
}

func (space *Workspace) AddOpened(doc *document.Document) error {
	if doc == nil || doc.Path == "" {
		return fmt.Errorf("preopened document has no disk path")
	}
	for index, existing := range space.Documents {
		if existing.Path == doc.Path {
			if existing != doc {
				_ = doc.Close()
			}
			space.Active = index
			return nil
		}
	}
	if len(space.Documents) == 1 && space.Current().Path == "" && !space.Current().Dirty() {
		space.Documents[0] = doc
		space.Active = 0
		return nil
	}
	space.Documents = append(space.Documents, doc)
	space.Active = len(space.Documents) - 1
	return nil
}

// Reload replaces the tab's document with a freshly opened one. The replacement
// carries its own document identity, editor and change-history source; the
// returned retired document keeps its own history and never transfers it to the
// replacement (H03 document lifecycle).
func (space *Workspace) Reload(index int) (*document.Document, error) {
	return space.reload(index, false)
}

func (space *Workspace) ReloadDiscard(index int) (*document.Document, error) {
	return space.reload(index, true)
}

func (space *Workspace) reload(index int, discard bool) (*document.Document, error) {
	if index < 0 || index >= len(space.Documents) {
		return nil, fmt.Errorf("tab index %d is out of range", index)
	}
	old := space.Documents[index]
	if old.Dirty() && !discard {
		return nil, ErrDirty
	}
	if old.Path == "" {
		return nil, fmt.Errorf("untitled tab has no disk path")
	}
	if err := old.CheckDiskPath(); err != nil {
		return nil, err
	}
	var updated *document.Document
	var err error
	if old.SavedEncoding() >= document.GB18030 {
		updated, err = document.OpenWithEncoding(old.Path, old.SavedEncoding())
	} else {
		updated, err = document.Open(old.Path)
	}
	if err != nil {
		return nil, err
	}
	if updated.Path != old.Path {
		_ = updated.Close()
		return nil, document.ErrDiskPathRedirected
	}
	space.Documents[index] = updated
	_ = old.Close()
	return old, nil
}

func (space *Workspace) Close(index int, discard bool) error {
	if index < 0 || index >= len(space.Documents) {
		return fmt.Errorf("tab index %d is out of range", index)
	}
	if space.Documents[index].Dirty() && !discard {
		return ErrDirty
	}
	_ = space.Documents[index].Close()
	space.Documents = slices.Delete(space.Documents, index, index+1)
	if len(space.Documents) == 0 {
		space.NewDocument()
	} else if space.Active > index {
		space.Active--
	} else if space.Active >= len(space.Documents) {
		space.Active = len(space.Documents) - 1
	}
	return nil
}
