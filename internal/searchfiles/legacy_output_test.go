package searchfiles

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"vivapad/internal/document"
)

func TestLegacyUnrepresentableOutputRejectedBeforeReplacementPreview(t *testing.T) {
	root := t.TempDir()
	files := []string{filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")}
	for _, file := range files {
		writeTestFile(t, file, []byte("target\r\n"))
	}
	options := Options{Pattern: "*.txt", Encoding: document.Windows1252}
	plan, err := PreviewReplace(context.Background(), root, "target", "\u4e2d", document.SearchOptions{}, options)
	if err == nil || plan != nil {
		t.Fatalf("unrepresentable Windows-1252 replacement offered a preview: %+v, %v", plan, err)
	}
	for _, file := range files {
		if data, err := os.ReadFile(file); err != nil || string(data) != "target\r\n" {
			t.Fatalf("unsafe preview changed %s: %x, %v", file, data, err)
		}
	}
	plan, err = PreviewReplace(context.Background(), root, "target", "\u00e9", document.SearchOptions{}, options)
	if err != nil || plan.Replacements != 2 {
		t.Fatalf("lossless replacement could not be retried: %+v, %v", plan, err)
	}
	result, err := plan.Apply(context.Background())
	if err != nil || result.FilesChanged != 2 || result.Replacements != 2 {
		t.Fatalf("lossless replacement failed: %+v, %v", result, err)
	}
	want := []byte{0xe9, 0x0d, 0x0a}
	for _, file := range files {
		if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, want) {
			t.Fatalf("lossless replacement changed expected bytes in %s: %x, %v", file, data, err)
		}
	}
}
