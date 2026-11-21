//go:build !unix

package recovery

import "os"

func openRecoveryFile(path string) (*os.File, error) {
	return os.Open(path)
}
