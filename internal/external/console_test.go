package external

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConsoleDirectoryUsesExistingParentWithoutOpeningDocument(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "draft & notes")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	document := filepath.Join(dir, "missing.txt")
	if got, err := consoleDirectory(document); got != dir || err != nil {
		t.Fatalf("containing directory = %q, %v, want %q", got, err, dir)
	}
	for _, path := range []string{"", "relative.txt", filepath.Join(dir, "missing", "draft.txt"), document + "\x00"} {
		if got, err := consoleDirectory(path); got != "" || err == nil {
			t.Errorf("invalid source %q resolved to %q, %v", path, got, err)
		}
	}
}
