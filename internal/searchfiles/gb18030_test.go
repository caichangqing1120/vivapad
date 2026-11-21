package searchfiles

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"vivapad/internal/document"
)

func TestGB18030PUAReplacementPreviewApplyAndReopen(t *testing.T) {
	root := t.TempDir()
	files := []string{filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")}
	original := []byte{'t', 'a', 'r', 'g', 'e', 't', '\r', '\n', 0xaa, 0xa1, '\n', 'B', '\r'}
	for _, file := range files {
		writeTestFile(t, file, original)
	}
	options := Options{Pattern: "*.txt", Encoding: document.GB18030}
	plan, err := PreviewReplace(context.Background(), root, "target", "\ue001\ue4c5", document.SearchOptions{}, options)
	if err != nil || plan == nil || plan.Replacements != 2 {
		t.Fatalf("preview=%+v, %v", plan, err)
	}
	for _, file := range files {
		if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, original) {
			t.Fatalf("preview wrote %s: %x, %v", file, data, err)
		}
	}
	result, err := plan.Apply(context.Background())
	if err != nil || result.FilesChanged != 2 || result.Replacements != 2 {
		t.Fatalf("apply=%+v, %v", result, err)
	}
	want := []byte{0xaa, 0xa2, 0xfe, 0xfe, '\r', '\n', 0xaa, 0xa1, '\n', 'B', '\r'}
	for _, file := range files {
		if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, want) {
			t.Fatalf("replacement=%x, %v; want %x", data, err, want)
		}
		doc, err := document.OpenWithEncoding(file, document.GB18030)
		if err != nil || doc.Editor.String() != "\ue001\ue4c5\n\ue000\nB\n" || doc.Dirty() {
			t.Fatalf("reopen=%+v, %v", doc, err)
		}
	}
}
