//go:build linux

package external_test

import (
	"crypto/sha256"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vivapad/internal/document"
	"vivapad/internal/external"
)

func TestLinuxNativeTrashMovesSavedDocumentIntoPrivateTrash(t *testing.T) {
	if os.Getenv("VIVAPAD_TEST_NATIVE_TRASH") != "1" {
		t.Skip("opt in to a real Linux gio trash operation on task-owned data")
	}
	root := t.TempDir()
	home, dataRoot := filepath.Join(root, "home"), filepath.Join(root, "data")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	path := filepath.Join(root, "task-owned-native-trash.txt")
	const content = "saved bytes \u4f60\u597d\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := doc.PrepareTrash()
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if err := plan.Apply(external.Trash); err != nil {
		t.Fatalf("real gio trash move: %v", err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists after trash: %v", err)
	}
	trashed := filepath.Join(dataRoot, "Trash", "files", filepath.Base(path))
	if got, err := os.ReadFile(trashed); err != nil || string(got) != content {
		t.Fatalf("private Trash bytes = %q, %v", got, err)
	}
	info, err := os.ReadFile(filepath.Join(dataRoot, "Trash", "info", filepath.Base(path)+".trashinfo"))
	if err != nil {
		t.Fatalf("private Trash metadata: %v", err)
	}
	if !strings.HasPrefix(string(info), "[Trash Info]\n") {
		t.Fatalf("invalid FreeDesktop Trash metadata: %q", info)
	}
	var original string
	for _, line := range strings.Split(string(info), "\n") {
		if strings.HasPrefix(line, "Path=") {
			original, err = url.PathUnescape(strings.TrimPrefix(line, "Path="))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if original != path || !strings.Contains(string(info), "\nDeletionDate=") {
		t.Fatalf("Trash metadata does not identify the source: %q", info)
	}
	t.Logf("private gio Trash retained the task file: sha256=%x metadata=%s", sha256.Sum256([]byte(content)), filepath.Base(path)+".trashinfo")
}

func TestLinuxNativeTrashUnavailableKeepsSavedDocument(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", filepath.Join(root, "no-executables"))
	t.Setenv("HOME", root)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	path := filepath.Join(root, "task-owned-unavailable.txt")
	const content = "saved bytes"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := doc.PrepareTrash()
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if err := plan.Apply(external.Trash); !errors.Is(err, external.ErrTrashUnsupported) {
		t.Fatalf("missing gio returned %v, want unavailable", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != content {
		t.Fatalf("unavailable backend changed the source: %q, %v", got, err)
	}
	if doc.Dirty() {
		t.Fatal("backend refusal dirtied the document")
	}
	t.Logf("missing gio refused without moving the task file: sha256=%x", sha256.Sum256([]byte(content)))
}
