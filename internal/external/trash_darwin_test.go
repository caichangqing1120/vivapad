//go:build darwin

package external

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeTrashMovesTaskFileAndReturnsCleanupPath(t *testing.T) {
	if os.Getenv("VIVAPAD_TEST_NATIVE_TRASH") != "1" {
		t.Skip("set VIVAPAD_TEST_NATIVE_TRASH=1 for a task-owned macOS Trash smoke")
	}
	root := t.TempDir()
	path := filepath.Join(root, filepath.Base(root)+"-vivapad-trash-smoke.txt")
	if err := os.WriteFile(path, []byte("task-owned"), 0600); err != nil {
		t.Fatal(err)
	}
	trashed, err := trashDarwin(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(trashed); err != nil {
			t.Errorf("remove task-owned Trash item: %v", err)
		}
	})
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists: %v", err)
	}
	if data, err := os.ReadFile(trashed); err != nil || string(data) != "task-owned" {
		t.Fatalf("recycled bytes = %q, %v", data, err)
	}
}
