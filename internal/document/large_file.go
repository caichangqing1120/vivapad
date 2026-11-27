package document

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/vivaui/viva/text"
	"vivapad/internal/largefile"
)

var (
	LightweightThreshold int64 = 16 << 20
	LargeFileThreshold   int64 = 64 << 20
	ErrLargeFileReadOnly       = errors.New("Large File (Read-Only): editing and saving are unavailable")
)

func openLarge(path string, selected *Encoding) (*Document, error) {
	if selected != nil {
		return nil, fmt.Errorf("Large File (Read-Only): open without an explicit legacy encoding to use the UTF-8 viewer")
	}
	file, err := largefile.Open(path)
	if err != nil {
		return nil, err
	}
	return &Document{
		Path: path, Name: filepath.Base(path), Editor: text.NewEditor(""), large: file,
		newline: "\n", savedNewline: "\n",
	}, nil
}

func (doc *Document) Lightweight() bool { return doc.lightweight }

func (doc *Document) LargeFile() *largefile.File { return doc.large }

func (doc *Document) Close() error {
	if doc.large != nil {
		return doc.large.Close()
	}
	return nil
}
