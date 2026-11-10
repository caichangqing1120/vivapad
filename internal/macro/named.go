package macro

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Named struct {
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

type namedDiskState struct {
	Version int     `json:"version"`
	Macros  []Named `json:"macros"`
}

func ValidateNamed(macros []Named) error {
	seen := make(map[string]bool, len(macros))
	for _, item := range macros {
		if item.Name == "" || strings.TrimSpace(item.Name) != item.Name || !utf8.ValidString(item.Name) || utf8.RuneCountInString(item.Name) > 80 {
			return fmt.Errorf("invalid macro name")
		}
		for _, character := range item.Name {
			if unicode.IsControl(character) {
				return fmt.Errorf("invalid macro name")
			}
		}
		key := strings.ToLower(item.Name)
		if seen[key] || len(item.Steps) == 0 {
			return fmt.Errorf("duplicate name or empty macro %q", item.Name)
		}
		seen[key] = true
		if err := Validate(item.Steps); err != nil {
			return fmt.Errorf("invalid macro %q: %w", item.Name, err)
		}
	}
	return nil
}

func SaveNamed(path string, macros []Named) error {
	if err := ValidateNamed(macros); err != nil {
		return err
	}
	data, err := json.Marshal(namedDiskState{Version: 1, Macros: macros})
	if err != nil {
		return err
	}
	if len(data) > maxBytes {
		return fmt.Errorf("named macros exceed 1 MiB")
	}
	return writePrivate(path, data)
}

func LoadNamed(path string) ([]Named, error) {
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
		return nil, fmt.Errorf("named macros exceed 1 MiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxBytes))
	decoder.DisallowUnknownFields()
	var stored namedDiskState
	if err := decoder.Decode(&stored); err != nil {
		return nil, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("named macros have trailing data")
	}
	if stored.Version != 1 {
		return nil, fmt.Errorf("unsupported named macro version %d", stored.Version)
	}
	if err := ValidateNamed(stored.Macros); err != nil {
		return nil, err
	}
	return stored.Macros, nil
}
