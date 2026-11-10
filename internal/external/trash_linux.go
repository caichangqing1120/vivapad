//go:build linux

package external

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func trashNative(path string) error {
	output, err := exec.Command("gio", "trash", "--", path).CombinedOutput()
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%w: gio trash", ErrTrashUnsupported)
	}
	if err != nil {
		return fmt.Errorf("gio trash: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
