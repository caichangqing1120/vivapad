package main

import (
	"os"

	"golang.org/x/sys/windows"
)

func writeLaunchUsage() error {
	stdout, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err == nil && stdout != 0 && stdout != windows.InvalidHandle {
		_, err := os.Stdout.WriteString(launchUsage)
		return err
	}

	// A GUI build started from a terminal can lack an inherited output handle.
	const attachParentProcess = ^uint32(0)
	attach := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
	if attached, _, err := attach.Call(uintptr(attachParentProcess)); attached == 0 && err != windows.ERROR_ACCESS_DENIED {
		return err
	}
	console, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer console.Close()
	_, err = console.WriteString(launchUsage)
	return err
}
