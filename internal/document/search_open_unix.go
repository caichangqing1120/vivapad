//go:build unix

package document

import (
	"os"

	"golang.org/x/sys/unix"
)

func openSearchFile(path string) (*os.File, error) {
	// The file may have become a pipe since the directory walk checked it.
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}
