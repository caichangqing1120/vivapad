package document

import (
	"errors"
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
// Uses the same Unicode and NFO detection as Open unless a legacy encoding is explicit.
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
