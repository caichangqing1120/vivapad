package main

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func creationTime(path string, info os.FileInfo) (time.Time, bool) {
	original, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	const fields = unix.STATX_BTIME | unix.STATX_INO
	var stat unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, 0, fields, &stat); err != nil || stat.Mask&fields != fields {
		return time.Time{}, false
	}
	// Do not pair a replacement's creation time with the original's modification time.
	if stat.Ino != original.Ino || unix.Mkdev(stat.Dev_major, stat.Dev_minor) != uint64(original.Dev) {
		return time.Time{}, false
	}
	return time.Unix(stat.Btime.Sec, int64(stat.Btime.Nsec)), true
}
