package external

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func LaunchEditor(path string) error {
	return launchEditor(path, os.Executable, func(program string, args ...string) error {
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

func launchEditor(path string, executable func() (string, error), run func(string, ...string) error) error {
	if !filepath.IsAbs(path) || strings.IndexByte(path, 0) >= 0 {
		return fmt.Errorf("selected file must be an absolute path without NUL")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("selected path is not a regular file: %s", path)
	}
	program, err := executable()
	if err != nil {
		return err
	}
	if !filepath.IsAbs(program) || strings.IndexByte(program, 0) >= 0 {
		return fmt.Errorf("editor executable path is invalid")
	}
	return run(program, path)
}

func RevealSelection(path string) error {
	return revealSelection(runtime.GOOS, path, func(program string, args ...string) error {
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

func revealSelection(goos, path string, run func(string, ...string) error) error {
	if !filepath.IsAbs(path) || strings.IndexByte(path, 0) >= 0 {
		return fmt.Errorf("selected path must be absolute text")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return fmt.Errorf("selected path is not a file or directory: %s", path)
	}
	switch goos {
	case "darwin":
		return run("open", "-R", path)
	case "windows":
		return run("explorer.exe", "/select,", path)
	case "linux":
		return run("xdg-open", filepath.Dir(path))
	default:
		return fmt.Errorf("file manager is unsupported on %s", goos)
	}
}
