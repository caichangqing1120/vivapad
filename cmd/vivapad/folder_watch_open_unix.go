//go:build unix

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func openFolderWatchDirectory(path string) (*os.File, error) {
	// The directory may have been replaced since the workspace listed it.
	return os.OpenFile(path, os.O_RDONLY|unix.O_DIRECTORY, 0)
}
