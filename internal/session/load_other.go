//go:build !unix

package session

import "os"

func openSessionFile(path string) (*os.File, error) {
	return os.Open(path)
}
