package document

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestMixedEOLSaveRefusesLossOfEmptyLine(t *testing.T) {
	for _, operation := range []string{"Save", "Save As", "Save Copy"} {
		t.Run(operation, func(t *testing.T) {
			original := []byte("first\rmiddle\nlast")
			path := filepath.Join(t.TempDir(), "original.txt")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			sourcePath := doc.Path
			doc.Editor.ReplaceRange(text.Range{Start: 6, End: 12}, "")
			if doc.Editor.String() != "first\n\nlast" || doc.EOLSequence() != "ML" || !doc.Dirty() {
				t.Fatalf("fixture: text %q, separators %q, dirty %v", doc.Editor.String(), doc.EOLSequence(), doc.Dirty())
			}
			destination := filepath.Join(filepath.Dir(path), "destination.txt")
			switch operation {
			case "Save":
				destination = path
				err = doc.Save("")
			case "Save As":
				err = doc.Save(destination)
			case "Save Copy":
				err = doc.SaveCopy(destination)
			}
			if err == nil || !strings.Contains(err.Error(), "CR and LF") {
				t.Fatalf("%s did not refuse the ambiguous separators: %v", operation, err)
			}
			if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, original) {
				t.Fatalf("original disk = %q, %v", data, err)
			}
			if destination != path {
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatalf("destination was created: %v", err)
				}
			}
			if doc.Editor.String() != "first\n\nlast" || doc.Path != sourcePath || !doc.Dirty() || !doc.Editor.Undo() || doc.Dirty() {
				t.Fatal("refused write changed the buffer, document path or save boundary")
			}
			if !doc.Editor.Redo() {
				t.Fatal("Redo failed")
			}
			doc.SetLineEnding(LF)
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(path)
			if err != nil || reopened.Editor.String() != "first\n\nlast" {
				t.Fatalf("explicit conversion failed to preserve the empty line: %v", err)
			}
		})
	}
}

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
