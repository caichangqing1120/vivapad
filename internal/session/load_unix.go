//go:build unix

package session

import (
	"os"

	"golang.org/x/sys/unix"
)

func openSessionFile(path string) (*os.File, error) {
	// A FIFO must not stall the open before the caller can reject it.
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}
