//go:build !windows

package external

func OpenConsole(ConsoleKind, string) error { return errConsoleUnsupported }
