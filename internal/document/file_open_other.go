//go:build !unix

package document

import "os"

func openDocumentFile(path string) (*os.File, error) {
	return os.Open(path)
}
