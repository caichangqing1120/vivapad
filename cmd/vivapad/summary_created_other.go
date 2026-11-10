//go:build !darwin && !windows

package main

import (
	"os"
	"time"
)

func creationTime(os.FileInfo) (time.Time, bool) { return time.Time{}, false }
