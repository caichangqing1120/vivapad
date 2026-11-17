package searchfiles

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"vivapad/internal/document"
)

func TestGB18030PUADirectoryReplaceAndReopen(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")}
	original := []byte{'t', 'a', 'r', 'g', 'e', 't', '\r', '\n', 0xaa, 0xa1, '\n', 'B', '\r'}
	for _, path := range paths {
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := PreviewReplace(context.Background(), root, "target", "\uE001\uE4C5", document.SearchOptions{}, Options{Pattern: "*.txt", Encoding: document.GB18030})
	if err != nil || plan == nil || plan.Replacements != 2 {
		t.Fatalf("PUA preview = %+v, %v", plan, err)
	}
	for _, path := range paths {
		if disk, err := os.ReadFile(path); err != nil || !bytes.Equal(disk, original) {
			t.Fatalf("preview wrote disk: %x, %v", disk, err)
		}
	}
	result, err := plan.Apply(context.Background())
	if err != nil || result.FilesChanged != 2 || result.Replacements != 2 {
		t.Fatalf("apply = %+v, %v", result, err)
	}
	want := []byte{0xaa, 0xa2, 0xfe, 0xfe, '\r', '\n', 0xaa, 0xa1, '\n', 'B', '\r'}
	for _, path := range paths {
		if disk, err := os.ReadFile(path); err != nil || !bytes.Equal(disk, want) {
			t.Fatalf("PUA replacement = %x, %v", disk, err)
		}
		doc, err := document.OpenWithEncoding(path, document.GB18030)
		if err != nil || doc.Editor.String() != "\uE001\uE4C5\n\uE000\nB\n" || doc.Dirty() {
			t.Fatalf("reopen PUA = %+v, %v", doc, err)
		}
	}
}
