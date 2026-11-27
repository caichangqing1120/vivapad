//go:build !windows

package shortcuts

import "os"

func replaceFile(source, target string) error { return os.Rename(source, target) }
