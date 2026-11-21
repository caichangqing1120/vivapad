package external

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestNamedBrowserReportsEarlyLauncherFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("exercise the Linux browser command lookup")
	}
	root := t.TempDir()
	path := filepath.Join(root, "saved draft.html")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "firefox"), []byte("#!/bin/sh\nexit 23\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root)
	err := OpenFileInNamedBrowser(Firefox, path)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("early browser failure was not reported: %v", err)
	}
}

func TestNamedBrowserReceivesOneFileURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test #100% & \u00e9.html")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			calls := 0
			err := openFileInNamedBrowser(goos, Firefox, path, func(name string) (string, error) {
				return "resolved/" + name, nil
			}, func(program string, args ...string) error {
				calls++
				wantProgram := "resolved/firefox"
				var prefix []string
				if goos == "darwin" {
					wantProgram, prefix = "/usr/bin/open", []string{"-b", "org.mozilla.firefox"}
				} else if goos == "windows" {
					wantProgram = "resolved/firefox.exe"
				}
				if program != wantProgram || len(args) != len(prefix)+1 || len(prefix) > 0 && !reflect.DeepEqual(args[:len(prefix)], prefix) {
					t.Fatalf("launch = %q %q", program, args)
				}
				target, err := url.Parse(args[len(args)-1])
				if err != nil || target.Scheme != "file" || target.Fragment != "" || target.RawQuery != "" || filepath.Base(target.Path) != filepath.Base(path) {
					t.Fatalf("file URL = %q, %v", args[len(args)-1], err)
				}
				return nil
			})
			if err != nil || calls != 1 {
				t.Fatalf("launch count %d, err %v", calls, err)
			}
		})
	}
}

func TestNamedBrowserRejectsUnsupportedAndInvalidFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "page.html")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		os      string
		browser NamedBrowser
		path    string
	}{
		{"darwin", InternetExplorer, path}, {"js", Firefox, path}, {"linux", Firefox, "relative.html"},
		{"linux", Chrome, root}, {"linux", Edge, filepath.Join(root, "missing.html")},
	} {
		if err := openFileInNamedBrowser(tc.os, tc.browser, tc.path, nil, func(string, ...string) error {
			t.Fatal("invalid request launched a process")
			return nil
		}); err == nil {
			t.Errorf("invalid target accepted: %+v", tc)
		}
	}
}

func TestBrowserFileURLPreservesDriveAndNetworkHosts(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"C:/Work/#preview%.html", "file:///C:/Work/%23preview%25.html"},
		{"//server/share/web page.html", "file://server/share/web%20page.html"},
	} {
		if got := browserFileURL(tc.path); got != tc.want {
			t.Errorf("file URL for %q = %q, want %q", tc.path, got, tc.want)
		}
	}
}
