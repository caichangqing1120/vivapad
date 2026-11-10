package recent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"vivapad/internal/document"
)

const (
	maxEntries = 10
	maxBytes   = 64 << 10
)

type Entry struct {
	Path     string            `json:"path"`
	Encoding document.Encoding `json:"encoding,omitempty"`
}

type diskState struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

func Add(entries []Entry, entry Entry) []Entry {
	result := make([]Entry, 0, maxEntries)
	result = append(result, entry)
	for _, existing := range entries {
		if existing.Path != entry.Path && len(result) < maxEntries {
			result = append(result, existing)
		}
	}
	return result
}

func validate(entries []Entry) error {
	if len(entries) > maxEntries {
		return fmt.Errorf("recent history exceeds %d entries", maxEntries)
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !filepath.IsAbs(entry.Path) || strings.IndexByte(entry.Path, 0) >= 0 || seen[entry.Path] {
			return fmt.Errorf("recent path is invalid or repeated: %q", entry.Path)
		}
		if entry.Encoding != document.UTF8 && !entry.Encoding.IsLegacy() {
			return fmt.Errorf("recent encoding is unsupported for %s", entry.Path)
		}
		seen[entry.Path] = true
	}
	return nil
}

func Save(path string, entries []Entry) error {
	if err := validate(entries); err != nil {
		return err
	}
	data, err := json.Marshal(diskState{Version: 1, Entries: entries})
	if err != nil {
		return err
	}
	if len(data) > maxBytes {
		return fmt.Errorf("recent history exceeds 64 KiB")
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("recent history directory is not a regular directory")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".vivapad-recent-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func Load(path string) ([]Entry, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("recent history exceeds 64 KiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxBytes))
	decoder.DisallowUnknownFields()
	var stored diskState
	if err := decoder.Decode(&stored); err != nil {
		return nil, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("recent history has trailing data")
	}
	if stored.Version != 1 {
		return nil, fmt.Errorf("unsupported recent history version %d", stored.Version)
	}
	if err := validate(stored.Entries); err != nil {
		return nil, err
	}
	return stored.Entries, nil
}

func Clear(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
