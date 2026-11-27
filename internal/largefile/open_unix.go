//go:build unix

package largefile

import (
	"os"

	"golang.org/x/sys/unix"
)

func openShared(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}
