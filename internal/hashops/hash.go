package hashops

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Algorithm uint8

const (
	MD5 Algorithm = iota + 1
	SHA1
	SHA256
	SHA512
)

func (algorithm Algorithm) String() string {
	switch algorithm {
	case MD5:
		return "MD5"
	case SHA1:
		return "SHA-1"
	case SHA256:
		return "SHA-256"
	case SHA512:
		return "SHA-512"
	}
	return "Unknown"
}

func newHash(algorithm Algorithm) (hash.Hash, error) {
	switch algorithm {
	case MD5:
		return md5.New(), nil
	case SHA1:
		return sha1.New(), nil
	case SHA256:
		return sha256.New(), nil
	case SHA512:
		return sha512.New(), nil
	}
	return nil, fmt.Errorf("unsupported hash algorithm %d", algorithm)
}

func Digest(algorithm Algorithm, data []byte) (string, error) {
	hasher, err := newHash(algorithm)
	if err != nil {
		return "", err
	}
	if _, err := hasher.Write(data); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func Text(algorithm Algorithm, value string, eachLine bool) (string, error) {
	if value == "" {
		return "", nil
	}
	if !eachLine {
		return Digest(algorithm, []byte(value))
	}
	var result strings.Builder
	var hasher hash.Hash
	var input []byte
	var sum [sha512.Size]byte
	var encoded [sha512.Size * 2]byte
	for line := range strings.Lines(value) {
		if strings.HasSuffix(line, "\n") {
			line = strings.TrimSuffix(line[:len(line)-1], "\r")
		}
		if line != "" {
			if hasher == nil {
				var err error
				hasher, err = newHash(algorithm)
				if err != nil {
					return "", err
				}
			}
			input = append(input[:0], line...)
			if _, err := hasher.Write(input); err != nil {
				return "", err
			}
			n := hex.Encode(encoded[:], hasher.Sum(sum[:0]))
			result.Write(encoded[:n])
			hasher.Reset()
		}
		result.WriteString("\r\n")
	}
	return result.String(), nil
}

func Files(algorithm Algorithm, paths []string) (string, error) {
	var result strings.Builder
	for _, path := range paths {
		hasher, err := newHash(algorithm)
		if err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("hash %s: %w", path, err)
		}
		info, err := file.Stat()
		if err == nil && !info.Mode().IsRegular() {
			err = fmt.Errorf("not a regular file")
		}
		if err == nil {
			_, err = io.Copy(hasher, file)
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return "", fmt.Errorf("hash %s: %w", path, err)
		}
		fmt.Fprintf(&result, "%x  %s\r\n", hasher.Sum(nil), filepath.Base(path))
	}
	return result.String(), nil
}
