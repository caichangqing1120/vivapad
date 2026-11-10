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
	return reveal(runtime.GOOS, path, func(program string, args ...string) error {
		command := exec.Command(program, args...)
		if err := command.Start(); err != nil {
			return err
		}
		if runtime.GOOS == "windows" {
			return command.Process.Release()
		}
		go func() { _ = command.Wait() }()
		return nil
	})
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
