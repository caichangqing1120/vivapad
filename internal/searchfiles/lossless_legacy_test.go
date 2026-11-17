package searchfiles

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"vivapad/internal/document"
)

func TestDirectoryPreviewRefusesLossyGB18030BeforeConfirmation(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "source.txt")
	original := []byte("target\r\n")
	if err := os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	options := Options{Pattern: "*.txt", Encoding: document.GB18030}
	plan, err := PreviewReplace(t.Context(), root, "target", "\uE000", document.SearchOptions{}, options)
	if err == nil || plan != nil {
		t.Fatalf("lossy preview offered for confirmation: plan=%+v err=%v", plan, err)
	}
	if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, original) {
		t.Fatalf("failed preview changed disk: %x, %v", disk, err)
	}
	plan, err = PreviewReplace(context.Background(), root, "target", "中", document.SearchOptions{}, options)
	if err != nil || plan == nil || plan.Replacements != 1 {
		t.Fatalf("lossless retry refused: plan=%+v err=%v", plan, err)
	}
	if _, err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, []byte{0xd6, 0xd0, '\r', '\n'}) {
		t.Fatalf("lossless replacement bytes: %x, %v", disk, err)
	}
}
