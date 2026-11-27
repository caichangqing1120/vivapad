package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
)

func ReadDiskRevision(path string) (string, error) {
	if err := CheckRecordedPath(path); err != nil {
		return "", err
	}
	file, err := openDocumentFile(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > LargeFileThreshold {
		return fmt.Sprintf("large:%d:%d", info.Size(), info.ModTime().UnixNano()), nil
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(file, LargeFileThreshold+1))
	if err != nil {
		return "", err
	}
	current, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !os.SameFile(info, current) || count != info.Size() || info.Size() != current.Size() || !info.ModTime().Equal(current.ModTime()) {
		return "", fmt.Errorf("%s changed while reading", path)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

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
