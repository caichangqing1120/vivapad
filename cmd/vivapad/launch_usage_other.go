//go:build !windows

package main

import "os"

func writeLaunchUsage() error {
	_, err := os.Stdout.WriteString(launchUsage)
	return err
}
