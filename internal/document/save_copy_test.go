package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveCopyPreservesSourceIdentityDirtyStateAndFormat(t *testing.T) {
	root := t.TempDir()
	source, copied := filepath.Join(root, "source.txt"), filepath.Join(root, "copy.txt")
	original := []byte{0xff, 0xfe, 0x2d, 0x4e, '\r', 0, '\n', 0}
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("文")
	path, name, digest := doc.Path, doc.Name, doc.DiskDigest()
	selection := doc.Editor.Selection()
	if err := doc.SaveCopy(copied); err != nil {
		t.Fatal(err)
	}
	want := append(append([]byte(nil), original...), 0x87, 0x65)
	if data, err := os.ReadFile(copied); err != nil || !bytes.Equal(data, want) {
		t.Fatalf("copy bytes = %x, err=%v; want %x", data, err, want)
	}
	if data, err := os.ReadFile(source); err != nil || !bytes.Equal(data, original) ||
		doc.Path != path || doc.Name != name || doc.DiskDigest() != digest || !doc.Dirty() || doc.Editor.Selection() != selection {
		t.Fatalf("copy mutated source/editor = %x, %v, %+v", data, err, doc)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "中\n" {
		t.Fatalf("copy damaged undo history: %q", doc.Editor.String())
	}
}

func TestSaveCopyFromUntitledLeavesDocumentUntitled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.txt")
	doc := New("new 1")
	doc.Editor.Insert("draft")
	if err := doc.SaveCopy(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "draft" || doc.Path != "" || doc.Name != "new 1" || !doc.Dirty() {
		t.Fatalf("untitled copy = %q, %v, doc=%+v", data, err, doc)
	}
}

func TestSaveCopyRefusesOwnPathAndDirectoryAlias(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	if err := os.WriteFile(source, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert(" local")
	if err := doc.SaveCopy(source); err == nil {
		t.Fatal("Save Copy overwrote the active source")
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if err := doc.SaveCopy(filepath.Join(alias, "source.txt")); err == nil {
		t.Fatal("Save Copy overwrote the source through a directory symlink")
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != "original" || !doc.Dirty() {
		t.Fatalf("rejected copy changed source: %q, %v", data, err)
	}
}

func TestSaveCopyEncodingErrorLeavesExistingTargetUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.txt")
	if err := os.WriteFile(path, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	doc := New("new 1")
	doc.Editor.Insert("中文")
	doc.SetEncoding(Windows1252)
	if err := doc.SaveCopy(path); err == nil {
		t.Fatal("unmappable Windows-1252 copy succeeded")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "untouched" || !doc.Dirty() {
		t.Fatalf("failed copy changed target: %q, %v", data, err)
	}
}
