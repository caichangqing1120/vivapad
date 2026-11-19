package document

import "strings"

// SearchSource binds normalized text to the file and encoding it came from.
type SearchSource struct {
	Path, Text, DiskDigest string
	Encoding               Encoding
}

var searchLineEndings = strings.NewReplacer("\r\n", "\n", "\r", "\n")

// ReadSearchSource reads a file without constructing an editor. With UTF8,
// Unicode input uses the same BOM detection as Open; other encodings are explicit.
func ReadSearchSource(path string, encoding Encoding) (SearchSource, error) {
	var selected *Encoding
	if encoding != UTF8 {
		selected = &encoding
	}
	file, err := readDecodedFile(path, selected)
	if err != nil {
		return SearchSource{}, err
	}
	return SearchSource{Path: file.path, Text: searchLineEndings.Replace(file.value),
		DiskDigest: digest(file.data), Encoding: file.encoding}, nil
}
