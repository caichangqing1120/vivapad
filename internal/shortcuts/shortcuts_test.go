package shortcuts

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPlatformBindings(t *testing.T) {
	chord := Chord{Primary | Alt | Shift, "F8"}
	for platform, want := range map[string]string{"darwin": "⌥⇧⌘F8", "windows": "Ctrl+Alt+Shift+F8", "linux": "Ctrl+Alt+Shift+F8"} {
		if got := chord.Format(platform); got != want {
			t.Errorf("%s: %q, want %q", platform, got, want)
		}
	}
}

func TestOverrideConflictPersistenceAndReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "shortcuts.json")
	catalog := append(Defaults(), Command{ID: "macro:Build", Name: "Macro: Build"})
	registry, err := Load(path, catalog)
	if err != nil {
		t.Fatal(err)
	}
	binding := Chord{Primary, "N"}
	var conflict *ConflictError
	if err := registry.Set("macro:Build", binding); !errors.As(err, &conflict) || conflict.Command != "New" {
		t.Fatalf("expected New conflict, got %v", err)
	}
	if err := registry.Set("New", Chord{}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Set("macro:Build", binding); err != nil {
		t.Fatal(err)
	}
	registry, err = Load(path, catalog)
	if err != nil || registry.Lookup(binding) != "macro:Build" || !registry.Binding("New").Empty() {
		t.Fatalf("override did not survive reopen: %v", err)
	}
	if err := registry.Reset("New"); !errors.As(err, &conflict) || conflict.Command != "Macro: Build" {
		t.Fatalf("reset must not overwrite a binding: %v", err)
	}
	if err := registry.ResetAll(); err != nil {
		t.Fatal(err)
	}
	registry, err = Load(path, catalog)
	if err != nil || registry.Lookup(binding) != "New" {
		t.Fatalf("reset not persisted: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("shortcut file permissions: %v", info.Mode())
	}
}

func TestBadStorageReturnsDefaults(t *testing.T) {
	for _, contents := range []string{`{`, `{"version":99}`, `{"version":1,"bindings":{"New":{"key":"bogus"}}}`, `{"version":1,"bindings":{"New":{"modifiers":1,"key":"S"}}}`} {
		path := filepath.Join(t.TempDir(), "shortcuts.json")
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		registry, err := Load(path, Defaults())
		if err == nil || registry.Lookup(Chord{Primary, "N"}) != "New" {
			t.Fatalf("bad storage %s did not fall back: %v", contents, err)
		}
	}
}

func TestFailedWriteLeavesBindingsUnchanged(t *testing.T) {
	path := t.TempDir()
	registry, err := Load(path, Defaults())
	if err == nil {
		t.Fatal("directory cannot be loaded as a file")
	}
	if err := registry.Set("New", Chord{Primary, "J"}); err == nil || registry.Binding("New") != (Chord{Primary, "N"}) {
		t.Fatalf("failed write changed active binding: %v", err)
	}
}

func TestNativeEditingChordsRemainReserved(t *testing.T) {
	registry, err := Load("", Defaults())
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Set("Copy", Chord{}); err == nil {
		t.Fatal("native Copy must not appear unassigned")
	}
	var conflict *ConflictError
	if err := registry.Set("New", Chord{Primary, "C"}); !errors.As(err, &conflict) || conflict.Command != "Copy" {
		t.Fatalf("native Copy must conflict: %v", err)
	}
}
