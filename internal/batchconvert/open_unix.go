//go:build unix

package batchconvert

import (
	"os"

	"golang.org/x/sys/unix"
)

func openRegular(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|unix.O_NONBLOCK, 0)
}
