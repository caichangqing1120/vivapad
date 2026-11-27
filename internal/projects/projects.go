package projects

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"vivapad/internal/privatejson"
)

const maxSize = 1 << 20

type Group struct {
	Name  string   `json:"name"`
	Files []string `json:"files,omitempty"`
}

func Load(path string) ([]Group, error) {
	var groups []Group
	if err := privatejson.Load(path, maxSize, &groups); err != nil {
		return nil, err
	}
	if err := validate(groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func Save(path string, groups []Group) error {
	if err := validate(groups); err != nil {
		return err
	}
	return privatejson.Save(path, maxSize, groups)
}

func FileKey(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func validate(groups []Group) error {
	names := make(map[string]bool)
	for _, group := range groups {
		key := strings.ToLower(group.Name)
		if key == "" || key != strings.TrimSpace(key) || len(key) > 96 || strings.ContainsFunc(key, unicode.IsControl) || names[key] {
			return fmt.Errorf("project names must be non-empty and unique (up to 96 bytes)")
		}
		names[key] = true
		files := make(map[string]bool)
		for _, path := range group.Files {
			key := FileKey(path)
			if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsRune(path, 0) || files[key] {
				return fmt.Errorf("invalid or repeated project file %q", path)
			}
			files[key] = true
		}
	}
	return nil
}
