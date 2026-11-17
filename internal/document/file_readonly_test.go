package document

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestToggleFileReadOnlyPreservesOtherPermissionsAndSaveSafety(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0664); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert(" changed")
	if readonly, err := ToggleFileReadOnly(path); err != nil || !readonly {
		t.Fatalf("set read-only = %v, %v", readonly, err)
	}
	if readonly, err := FileReadOnly(path); err != nil || !readonly {
		t.Fatalf("read-only state = %v, %v", readonly, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0222 != 0 || info.Mode().Perm()&0444 != 0444 {
		t.Fatalf("file mode after read-only = %v, %v", info.Mode(), err)
	}
	if err := doc.Save(""); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("read-only Save = %v, want permission error", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "original" || !doc.Dirty() {
		t.Fatalf("read-only Save changed file or buffer: %q, %v, dirty=%v", data, err, doc.Dirty())
	}
	if readonly, err := ToggleFileReadOnly(path); err != nil || readonly {
		t.Fatalf("clear read-only = %v, %v", readonly, err)
	}
	info, err = os.Stat(path)
	// Posix clears and then restores only owner-write. Windows clears the
	// read-only attribute instead, and Go reports the resulting 0666 there, so
	// the platform property to pin is the attribute, not the mode bits.
	if runtime.GOOS == "windows" {
		if err != nil || info.Mode().Perm()&0200 == 0 {
			t.Fatalf("file attribute after clearing read-only = %v, %v", info.Mode(), err)
		}
	} else if err != nil || info.Mode().Perm()&0222 != 0200 {
		t.Fatalf("file mode after clearing read-only = %v, %v", info.Mode(), err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatalf("Save after clearing read-only: %v", err)
	}
}

func TestToggleFileReadOnlyRejectsInvalidTargetsAndChmodFailure(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"", root, filepath.Join(root, "missing.txt")} {
		if _, err := ToggleFileReadOnly(target); err == nil {
			t.Errorf("invalid file %q accepted", target)
		}
	}
	want := errors.New("chmod denied")
	readonly, err := toggleFileReadOnly(path, func(string, os.FileMode) error { return want })
	if !errors.Is(err, want) || readonly {
		t.Fatalf("failed chmod = %v, %v", readonly, err)
	}
	if current, err := FileReadOnly(path); err != nil || current {
		t.Fatalf("failed chmod changed mode: %v, %v", current, err)
	}
}

func TestSaveAsFromFileReadOnlyDocumentKeepsOriginalBytes(t *testing.T) {
	root := t.TempDir()
	original, copy := filepath.Join(root, "original.txt"), filepath.Join(root, "copy.txt")
	if err := os.WriteFile(original, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(original)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert("after ")
	if _, err := ToggleFileReadOnly(original); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(copy); err != nil {
		t.Fatalf("Save As from read-only source: %v", err)
	}
	if data, err := os.ReadFile(original); err != nil || string(data) != "before" {
		t.Fatalf("Save As changed read-only source: %q, %v", data, err)
	}
	if data, err := os.ReadFile(copy); err != nil || string(data) != "after before" {
		t.Fatalf("Save As copy = %q, %v", data, err)
	}
}
