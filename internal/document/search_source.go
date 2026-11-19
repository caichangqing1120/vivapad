package document

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

// SearchSource binds normalized text to the file and encoding it came from.
type SearchSource struct {
	Path, Text, DiskDigest string
	Encoding               Encoding
}

var (
	ErrSearchFileTooLarge = errors.New("file exceeds search size limit")
	searchLineEndings     = strings.NewReplacer("\r\n", "\n", "\r", "\n")
)

// ReadSearchSource reads a size-bounded file without constructing an editor.
// Unicode uses BOM detection; legacy code pages are explicit.
func ReadSearchSource(path string, encoding Encoding, maxBytes int64) (SearchSource, error) {
	file, err := readDecodedFile(path, searchEncoding(encoding), maxBytes)
	if err != nil {
		return SearchSource{}, err
	}
	return SearchSource{Path: file.path, Text: searchLineEndings.Replace(file.value),
		DiskDigest: digest(file.data), Encoding: file.encoding}, nil
}

// OpenSearchDocument opens a size-bounded editing draft for Replace in Files.
func OpenSearchDocument(path string, encoding Encoding, maxBytes int64) (*Document, error) {
	return open(path, searchEncoding(encoding), maxBytes)
}

func searchEncoding(encoding Encoding) *Encoding {
	if encoding.IsLegacy() {
		return &encoding
	}
	return nil
}

func readSearchFile(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > maxBytes {
		return nil, ErrSearchFileTooLarge
	}
	var data bytes.Buffer
	if info.Size() <= math.MaxInt-bytes.MinRead {
		data.Grow(int(info.Size()) + bytes.MinRead)
	}
	var reader io.Reader = file
	if maxBytes < math.MaxInt64 {
		// Read one extra byte to detect growth after Stat without reading to EOF.
		reader = io.LimitReader(file, maxBytes+1)
	}
	if count, err := data.ReadFrom(reader); err != nil {
		return nil, err
	} else if count > maxBytes {
		return nil, ErrSearchFileTooLarge
	}
	return data.Bytes(), nil
}
