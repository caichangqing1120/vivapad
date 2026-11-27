package external

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func Reveal(path string) error {
	return reveal(runtime.GOOS, path, launchFileManager)
}

func launchFileManager(program string, args ...string) error {
	command := exec.Command(program, args...)
	if err := command.Start(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return command.Process.Release()
	}
	go func() { _ = command.Wait() }()
	return nil
}

func OpenFolder(path string) error {
	return openFolder(runtime.GOOS, path, launchFileManager)
}

func openFolder(goos, path string, run func(string, ...string) error) error {
	if strings.IndexByte(path, 0) >= 0 || !filepath.IsAbs(path) {
		return fmt.Errorf("file-manager path must be absolute text")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || strings.HasSuffix(strings.ToLower(path), ".app") {
		return fmt.Errorf("file-manager target is not a regular directory: %s", path)
	}
	switch goos {
	case "darwin":
		return run("open", "-a", "Finder", path)
	case "windows":
		return run("explorer.exe", path)
	case "linux":
		return run("xdg-open", path)
	default:
		return fmt.Errorf("file manager is unsupported on %s", goos)
	}
}

func reveal(goos, path string, run func(string, ...string) error) error {
	if strings.IndexByte(path, 0) >= 0 || !filepath.IsAbs(path) {
		return fmt.Errorf("file-manager path must be absolute text")
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	if !missing && !info.Mode().IsRegular() {
		return fmt.Errorf("file-manager target is not a regular file: %s", path)
	}
	if missing {
		parentInfo, err := os.Stat(parent)
		if err != nil {
			return err
		}
		if !parentInfo.IsDir() {
			return fmt.Errorf("file-manager parent is not a directory: %s", parent)
		}
	}
	switch goos {
	case "darwin":
		if missing {
			return run("open", parent)
		}
		return run("open", "-R", path)
	case "windows":
		if missing {
			return run("explorer.exe", parent)
		}
		return run("explorer.exe", "/select,", path)
	case "linux":
		return run("xdg-open", parent)
	default:
		return fmt.Errorf("file manager is unsupported on %s", goos)
	}
}
