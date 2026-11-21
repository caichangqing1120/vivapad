package external

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type NamedBrowser string

const (
	Firefox          NamedBrowser = "Firefox"
	Chrome           NamedBrowser = "Chrome"
	Edge             NamedBrowser = "Edge"
	InternetExplorer NamedBrowser = "Internet Explorer"
)

type browserEntry struct {
	bundle, executable string
	commands           []string
}

var namedBrowsers = map[NamedBrowser]browserEntry{
	Firefox:          {"org.mozilla.firefox", "firefox.exe", []string{"firefox"}},
	Chrome:           {"com.google.Chrome", "chrome.exe", []string{"google-chrome", "google-chrome-stable"}},
	Edge:             {"com.microsoft.edgemac", "msedge.exe", []string{"microsoft-edge", "microsoft-edge-stable"}},
	InternetExplorer: {"", "iexplore.exe", nil},
}

func NamedBrowserSupported(browser NamedBrowser) bool {
	if browser == InternetExplorer {
		return legacyInternetExplorer()
	}
	_, ok := namedBrowsers[browser]
	return ok && (runtime.GOOS == "darwin" || runtime.GOOS == "linux" || runtime.GOOS == "windows")
}

func OpenFileInNamedBrowser(browser NamedBrowser, path string) error {
	if !NamedBrowserSupported(browser) {
		return fmt.Errorf("%s is unavailable on this platform", browser)
	}
	lookup := exec.LookPath
	if runtime.GOOS == "windows" {
		lookup = lookupWindowsBrowser
	}
	return openFileInNamedBrowser(runtime.GOOS, browser, path, lookup, func(program string, args ...string) error {
		if runtime.GOOS == "darwin" {
			// Launch Services reports an uninstalled bundle through open's exit status.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := exec.CommandContext(ctx, program, args...).Run(); err != nil {
				return fmt.Errorf("could not open %s: %w", browser, err)
			}
			return nil
		}
		command := exec.Command(program, args...)
		if err := command.Start(); err != nil {
			return err
		}
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		// A browser can hand off to an existing process or stay running. Wait
		// briefly for startup failures without holding or killing the browser.
		startup := time.NewTimer(500 * time.Millisecond)
		defer startup.Stop()
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("browser exited during launch: %w", err)
			}
			return nil
		case <-startup.C:
			return nil
		}
	})
}

func openFileInNamedBrowser(goos string, browser NamedBrowser, path string, lookup func(string) (string, error), run func(string, ...string) error) error {
	entry, ok := namedBrowsers[browser]
	if !ok || browser == InternetExplorer && goos != "windows" || goos != "darwin" && goos != "linux" && goos != "windows" {
		return fmt.Errorf("%s is unavailable on this platform", browser)
	}
	if !filepath.IsAbs(path) || strings.IndexByte(path, 0) >= 0 {
		return errors.New("browser target must be an absolute file path")
	}
	path = filepath.Clean(path)
	normalized := filepath.ToSlash(path)
	if strings.HasPrefix(normalized, "//?/") || strings.HasPrefix(normalized, "//./") {
		return errors.New("browser target must not use a Windows device path")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("browser target is not a regular file")
	}
	target := browserFileURL(normalized)
	if goos == "darwin" {
		return run("/usr/bin/open", "-b", entry.bundle, target)
	}
	commands := entry.commands
	if goos == "windows" {
		commands = []string{entry.executable}
	}
	for _, name := range commands {
		if program, err := lookup(name); err == nil {
			return run(program, target)
		}
	}
	return fmt.Errorf("%s is not installed or registered", browser)
}

func browserFileURL(path string) string {
	target := url.URL{Scheme: "file", Path: path}
	if host, tail, ok := strings.Cut(strings.TrimPrefix(path, "//"), "/"); strings.HasPrefix(path, "//") && ok && host != "" {
		target.Host, target.Path = host, "/"+tail
	} else if !strings.HasPrefix(path, "/") {
		target.Path = "/" + path
	}
	return target.String()
}
