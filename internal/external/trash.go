package external

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrTrashUnsupported = errors.New("system trash is unavailable")

func Trash(path string) error {
	if err := validateTrashPath(path); err != nil {
		return err
	}
	return trashNative(path)
}

func validateTrashPath(path string) error {
	if !filepath.IsAbs(path) || strings.IndexByte(path, 0) >= 0 {
		return fmt.Errorf("trash path must be absolute text")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("trash target is not a regular file: %s", path)
	}
	return nil
}
