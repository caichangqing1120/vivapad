package external

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLaunchEditorPreflightsFileAndPassesOneArgument(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "note #1; quoted.txt")
	if err := os.WriteFile(file, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	var got []string
	launch := func(program string, args ...string) error {
		got = append([]string{program}, args...)
		return nil
	}
	if err := launchEditor(file, func() (string, error) { return filepath.Join(root, "VivaPad"), nil }, launch); err != nil ||
		!reflect.DeepEqual(got, []string{filepath.Join(root, "VivaPad"), file}) {
		t.Fatalf("new instance args = %v, %v", got, err)
	}
	got = nil
	for _, invalid := range []string{"relative.txt", root, filepath.Join(root, "missing.txt"), file + "\x00"} {
		if err := launchEditor(invalid, func() (string, error) { return filepath.Join(root, "VivaPad"), nil }, launch); err == nil {
			t.Fatalf("unsafe selected file %q accepted", invalid)
		}
	}
	if got != nil {
		t.Fatalf("invalid selected file started a process: %v", got)
	}
	want := errors.New("launch denied")
	if err := launchEditor(file, func() (string, error) { return filepath.Join(root, "VivaPad"), nil }, func(string, ...string) error { return want }); !errors.Is(err, want) {
		t.Fatalf("launch error = %v", err)
	}
}

func TestRevealSelectedFileOrDirectoryUsesManagerWithoutShell(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a name #1.txt")
	if err := os.WriteFile(file, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		goos, path string
		want       []string
	}{
		{"darwin", file, []string{"open", "-R", file}},
		{"darwin", root, []string{"open", "-R", root}},
		{"windows", root, []string{"explorer.exe", "/select,", root}},
		{"linux", file, []string{"xdg-open", root}},
		{"linux", root, []string{"xdg-open", filepath.Dir(root)}},
	} {
		var got []string
		err := revealSelection(item.goos, item.path, func(program string, args ...string) error {
			got = append([]string{program}, args...)
			return nil
		})
		if err != nil || !reflect.DeepEqual(got, item.want) {
			t.Errorf("%s reveal %q = %v, %v; want %v", item.goos, item.path, got, err, item.want)
		}
	}
	called := false
	for _, invalid := range []string{filepath.Join(root, "missing"), "relative", root + "\x00"} {
		if err := revealSelection("darwin", invalid, func(string, ...string) error { called = true; return nil }); err == nil {
			t.Fatalf("invalid reveal %q accepted", invalid)
		}
	}
	if called {
		t.Fatal("invalid selected path launched a file manager")
	}
}
