package external

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConsoleKind uint8

const (
	CommandPrompt ConsoleKind = iota
	WindowsPowerShell
)

var errConsoleUnsupported = errors.New("Windows consoles are unavailable on this platform")

func consoleDirectory(path string) (string, error) {
	if path == "" {
		return "", errors.New("save the document before opening its folder")
	}
	if !filepath.IsAbs(path) || strings.IndexByte(path, 0) >= 0 {
		return "", errors.New("document path must be absolute text")
	}
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("containing folder is unavailable: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("containing folder is not a directory")
	}
	return dir, nil
}
