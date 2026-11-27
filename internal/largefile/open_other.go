//go:build !unix && !windows

package largefile

import "os"

func openShared(path string) (*os.File, error) { return os.Open(path) }
