//go:build unix

package batchconvert

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSkipFIFOAndPreservePermissions(t *testing.T) {
	root := t.TempDir()
	if err := unix.Mkfifo(filepath.Join(root, "a.txt"), 0600); err != nil {
		t.Fatal(err)
	}
	path := fixture(t, root, "b.txt", "change\r")
	if err := os.Chmod(path, 0751); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(t.Context(), Options{Root: root, TargetEOL: LF})
	if err != nil {
		t.Fatal(err)
	}
	if entries := plan.Entries(); len(entries) != 2 || entries[0].Action != Skip || entries[0].Reason != ReasonSpecial {
		t.Fatalf("preview: %+v", entries)
	}
	if result := Apply(t.Context(), plan); result.Converted != 1 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("apply: %+v", result)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0751 {
		t.Fatalf("permissions: %v, %v", info, err)
	}
	if children, err := os.ReadDir(root); err != nil || len(children) != 2 {
		t.Fatalf("temporary files were left behind: %+v, %v", children, err)
	}
}
