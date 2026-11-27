package external

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRevealUsesPlatformFileManagerWithoutShellInterpolation(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a name #1.txt")
	if err := os.WriteFile(file, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		goos string
		want []string
	}{
		{"darwin", []string{"open", "-R", file}},
		{"windows", []string{"explorer.exe", "/select,", file}},
		{"linux", []string{"xdg-open", root}},
	} {
		var got []string
		err := reveal(test.goos, file, func(program string, args ...string) error {
			got = append([]string{program}, args...)
			return nil
		})
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s reveal = %v, %v; want %v", test.goos, got, err, test.want)
		}
	}
}

func TestOpenFolderUsesPlatformFileManager(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config & preferences")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	for goos, want := range map[string][]string{
		"darwin": {"open", "-a", "Finder", path}, "windows": {"explorer.exe", path}, "linux": {"xdg-open", path},
	} {
		var got []string
		err := openFolder(goos, path, func(program string, args ...string) error {
			got = append([]string{program}, args...)
			return nil
		})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s open = %v, %v; want %v", goos, got, err, want)
		}
	}
}

func TestRevealMissingFileOpensExistingParentAndRejectsUnsafeTargets(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "deleted.txt")
	var got []string
	err := reveal("darwin", missing, func(program string, args ...string) error {
		got = append([]string{program}, args...)
		return nil
	})
	if err != nil || !reflect.DeepEqual(got, []string{"open", root}) {
		t.Fatalf("missing-file fallback = %v, %v", got, err)
	}
	called := false
	for _, path := range []string{"relative.txt", root, filepath.Join(root, "no-parent", "missing.txt"), "/tmp/has\x00nul"} {
		if err := reveal("darwin", path, func(string, ...string) error { called = true; return nil }); err == nil {
			t.Errorf("unsafe path %q was accepted", path)
		}
	}
	if called {
		t.Fatal("unsafe reveal called the file manager")
	}
	want := errors.New("file manager failed")
	if err := reveal("darwin", missing, func(string, ...string) error { return want }); !errors.Is(err, want) {
		t.Fatalf("file-manager error was lost: %v", err)
	}
}
