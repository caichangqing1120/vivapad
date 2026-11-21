//go:build unix

package document

import (
	"os"

	"golang.org/x/sys/unix"
)

func openDocumentFile(path string) (*os.File, error) {
	// A path may name a pipe by the time we open it and check its type.
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}
