//go:build !unix

package document

import "os"

func openSearchFile(path string) (*os.File, error) {
	return os.Open(path)
}
