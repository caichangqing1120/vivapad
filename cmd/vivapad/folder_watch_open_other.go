//go:build !unix

package main

import "os"

func openFolderWatchDirectory(path string) (*os.File, error) {
	return os.Open(path)
}
