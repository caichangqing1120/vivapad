package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestMixedEOLSaveUndoRetainsDeletedLFLabel(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	original := []byte("a\r\nb\nc\rd")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != "a\r\nbc\rd" {
		t.Fatalf("edited Save bytes = %q, %v", disk, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\nb\nc\nd" {
		t.Fatal("Undo did not restore the text")
	}
	copyPath := filepath.Join(root, "undone.txt")
	if err := doc.SaveCopy(copyPath); err != nil {
		t.Fatal(err)
	}
	if disk, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(disk, original) {
		t.Fatalf("Undo lost the original physical separators: %q, %v", disk, err)
	}
	if !doc.Editor.Redo() {
		t.Fatal("Redo failed")
	}
	if err := doc.SaveCopy(copyPath); err != nil {
		t.Fatal(err)
	}
	if disk, err := os.ReadFile(copyPath); err != nil || string(disk) != "a\r\nbc\rd" {
		t.Fatalf("Redo lost the edited physical separators: %q, %v", disk, err)
	}
}

func TestMixedEOLSeparatedSavesRetainOriginalHistory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	original := []byte("a\r\nb\nc\rd")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 2, End: 2}, "new\n")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(root, "history.txt")
	for _, want := range []string{"a\r\nbc\rd", string(original)} {
		if !doc.Editor.Undo() {
			t.Fatal("Undo failed")
		}
		if err := doc.SaveCopy(copyPath); err != nil {
			t.Fatal(err)
		}
		if disk, err := os.ReadFile(copyPath); err != nil || string(disk) != want {
			t.Fatalf("Undo physical bytes = %q, want %q, error=%v", disk, want, err)
		}
	}
}

func TestMixedEOLExplicitConversionRelabelsUndoHistory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	if err := os.WriteFile(path, []byte("a\r\nb\nc\rd"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if !doc.Editor.Undo() {
		t.Fatal("Undo failed")
	}
	doc.SetLineEnding(LF)
	copyPath := filepath.Join(root, "uniform.txt")
	for _, want := range []string{"a\nb\nc\nd", "a\nbc\nd"} {
		if err := doc.SaveCopy(copyPath); err != nil {
			t.Fatal(err)
		}
		if disk, err := os.ReadFile(copyPath); err != nil || string(disk) != want {
			t.Fatalf("converted physical bytes = %q, want %q, error=%v", disk, want, err)
		}
		if want == "a\nb\nc\nd" && !doc.Editor.Redo() {
			t.Fatal("Redo failed")
		}
	}
}
