package external

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrashValidatesRegularAbsolutePathWithoutFollowingSymlink(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a name #1.txt")
	if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative.txt", root, link, filepath.Join(root, "missing.txt"), "/tmp/bad\x00name"} {
		if err := validateTrashPath(path); err == nil {
			t.Errorf("unsafe trash path %q was accepted", path)
		}
	}
	if err := validateTrashPath(file); err != nil {
		t.Fatalf("regular file rejected: %v", err)
	}
}
