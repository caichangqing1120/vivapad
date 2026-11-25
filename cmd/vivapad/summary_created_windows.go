//go:build windows

package main

import (
	"os"
	"syscall"
	"time"
)

func creationTime(_ string, info os.FileInfo) (time.Time, bool) {
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || stat.CreationTime.Nanoseconds() <= 0 {
		return time.Time{}, false
	}
	return time.Unix(0, stat.CreationTime.Nanoseconds()), true
}
