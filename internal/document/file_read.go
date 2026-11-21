package document

import (
	"bytes"
	"fmt"
	"io"
	"math"
)

func readDocumentFile(path string, maxBytes int64) ([]byte, error) {
	file, err := openDocumentFile(path)
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
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil, ErrSearchFileTooLarge
	}
	var data bytes.Buffer
	if info.Size() <= math.MaxInt-bytes.MinRead {
		data.Grow(int(info.Size()) + bytes.MinRead)
	}
	var reader io.Reader = file
	if maxBytes > 0 && maxBytes < math.MaxInt64 {
		// Read one extra byte to detect growth after Stat without reading to EOF.
		reader = io.LimitReader(file, maxBytes+1)
	}
	if count, err := data.ReadFrom(reader); err != nil {
		return nil, err
	} else if maxBytes > 0 && count > maxBytes {
		return nil, ErrSearchFileTooLarge
	}
	return data.Bytes(), nil
}
