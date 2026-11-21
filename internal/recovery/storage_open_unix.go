//go:build unix

package recovery

import (
	"os"

	"golang.org/x/sys/unix"
)

func openRecoveryFile(path string) (*os.File, error) {
	// A named pipe must not hold up startup before Load can check its type.
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}
