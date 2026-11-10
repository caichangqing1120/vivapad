package runcommand

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxNamedBytes = 1 << 20

type Named struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

type namedDisk struct {
	Version  int     `json:"version"`
	Commands []Named `json:"commands"`
}

func ValidateNamed(commands []Named) error {
	if len(commands) > 64 {
		return fmt.Errorf("too many Run commands")
	}
	seen := make(map[string]bool, len(commands))
	for _, command := range commands {
		if command.Name == "" || strings.TrimSpace(command.Name) != command.Name || !utf8.ValidString(command.Name) || utf8.RuneCountInString(command.Name) > 80 || strings.IndexFunc(command.Name, unicode.IsControl) >= 0 {
			return fmt.Errorf("invalid Run command name")
		}
		key := strings.ToLower(command.Name)
		if seen[key] {
			return fmt.Errorf("duplicate Run command %q", command.Name)
		}
		seen[key] = true
		if _, err := splitTemplate(command.Template); err != nil {
			return fmt.Errorf("invalid Run command %q: %w", command.Name, err)
		}
	}
	return nil
}

func Save(path string, commands []Named) error {
	if err := ValidateNamed(commands); err != nil {
		return err
	}
	data, err := json.Marshal(namedDisk{Version: 1, Commands: commands})
	if err != nil {
		return err
	}
	if len(data) > maxNamedBytes {
		return fmt.Errorf("named Run commands exceed 1 MiB")
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
		return fmt.Errorf("Run command directory is not regular")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".vivapad-run-*")
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

func Load(path string) ([]Named, error) {
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
	if info.Size() > maxNamedBytes {
		return nil, fmt.Errorf("named Run commands exceed 1 MiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxNamedBytes))
	decoder.DisallowUnknownFields()
	var stored namedDisk
	if err := decoder.Decode(&stored); err != nil {
		return nil, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("named Run commands have trailing data")
	}
	if stored.Version != 1 {
		return nil, fmt.Errorf("unsupported named Run version %d", stored.Version)
	}
	if err := ValidateNamed(stored.Commands); err != nil {
		return nil, err
	}
	return stored.Commands, nil
}
