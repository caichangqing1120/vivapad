package foldertree

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Entry struct {
	Path      string
	Name      string
	Directory bool
}

func CanonicalRoot(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", resolved)
	}
	return filepath.Clean(resolved), nil
}

// CheckCanonicalPath verifies a path that was already resolved when recorded.
func CheckCanonicalPath(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if resolved != filepath.Clean(path) {
		return fmt.Errorf("path now points elsewhere: %s", path)
	}
	return nil
}

func Contains(root, path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, resolved)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func Children(path string) ([]Entry, error) {
	contents, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	children := make([]Entry, 0, len(contents))
	for _, item := range contents {
		if item.Type()&os.ModeSymlink != 0 || item.IsDir() && strings.HasPrefix(item.Name(), ".") {
			continue
		}
		if !item.IsDir() && !item.Type().IsRegular() {
			info, err := item.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
		}
		children = append(children, Entry{Path: filepath.Join(path, item.Name()), Name: item.Name(), Directory: item.IsDir()})
	}
	slices.SortFunc(children, func(first, second Entry) int {
		if first.Directory != second.Directory {
			if first.Directory {
				return -1
			}
			return 1
		}
		if order := strings.Compare(strings.ToLower(first.Name), strings.ToLower(second.Name)); order != 0 {
			return order
		}
		return strings.Compare(first.Name, second.Name)
	})
	return children, nil
}
