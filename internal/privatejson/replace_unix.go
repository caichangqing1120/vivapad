//go:build !windows

package privatejson

import "os"

func Replace(source, target string) error { return os.Rename(source, target) }
