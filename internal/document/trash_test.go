package document

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTrashPlanRejectsUntitledAndChangedSource(t *testing.T) {
	if _, err := New("new 1").PrepareTrash(); err == nil {
		t.Fatal("untitled document has no disk file to trash")
	}
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("external edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := doc.PrepareTrash(); !errors.Is(err, ErrTrashSourceChanged) {
		t.Fatalf("changed source = %v", err)
	}
}

func TestTrashPlanRechecksIdentityAndContentBeforeMoving(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := doc.PrepareTrash()
	if err != nil {
		t.Fatal(err)
	}
	called := false
	move := func(string) error { called = true; return nil }
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(move); !errors.Is(err, ErrTrashSourceChanged) || called {
		t.Fatalf("changed file moved: %v, called=%v", err, called)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(move); !errors.Is(err, ErrTrashSourceChanged) || called {
		t.Fatalf("replacement moved: %v, called=%v", err, called)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "backup.txt")
	if err := os.WriteFile(backup, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(backup, path); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(move); !errors.Is(err, ErrTrashSourceChanged) || called {
		t.Fatalf("symlink moved: %v, called=%v", err, called)
	}
}

func TestTrashPlanKeepsDirtyBufferAndChecksBackendResult(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	canonicalPath := doc.Path
	doc.Editor.Insert("draft ")
	plan, err := doc.PrepareTrash()
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("trash unavailable")
	if err := plan.Apply(func(string) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("backend failure = %v", err)
	}
	if err := plan.Apply(func(string) error { return nil }); !errors.Is(err, ErrTrashSourceRemains) {
		t.Fatalf("backend reported success without moving = %v", err)
	}
	if err := plan.Apply(func(source string) error { return os.Rename(source, filepath.Join(root, "moved.txt")) }); err != nil {
		t.Fatal(err)
	}
	if !doc.Dirty() || doc.Editor.String() != "draft original" || doc.Path != canonicalPath {
		t.Fatalf("trash mutated document before caller closes it: path=%q dirty=%v value=%q", doc.Path, doc.Dirty(), doc.Editor.String())
	}
}
