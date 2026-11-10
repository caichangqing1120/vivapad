package document

import (
	"fmt"
	"os"
)

func FileReadOnly(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("choose a saved file")
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("not a regular file: %s", path)
	}
	return info.Mode().Perm()&0222 == 0, nil
}

func ToggleFileReadOnly(path string) (bool, error) {
	return toggleFileReadOnly(path, os.Chmod)
}

func toggleFileReadOnly(path string, chmod func(string, os.FileMode) error) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("choose a saved file")
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("not a regular file: %s", path)
	}
	mode := info.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	if mode.Perm()&0222 == 0 {
		mode |= 0200
	} else {
		mode &^= 0222
	}
	if err := chmod(path, mode); err != nil {
		return false, err
	}
	return FileReadOnly(path)
}
