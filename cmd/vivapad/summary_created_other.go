//go:build !darwin && !linux && !windows

package main

import (
	"os"
	"time"
)

func creationTime(string, os.FileInfo) (time.Time, bool) { return time.Time{}, false }
