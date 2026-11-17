package document

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// retireTrashSource moves the prepared source out of the way so a fixture can
// write the replacement at its path. Windows keeps a deleted-but-open name
// delete-pending until the plan's descriptor closes, so it renames the original
// aside instead of unlinking it; POSIX unlinks, which keeps the inode-recycling
// case the tests cover. The plan must already hold its descriptor.
func retireTrashSource(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if err := os.Rename(path, path+".trashed"); err != nil {
			t.Fatalf("retire source: %v", err)
		}
		return
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("retire source: %v", err)
	}
}

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
	open := func(t *testing.T) *Document {
		t.Helper()
		if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
		doc, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		return doc
	}
	prepare := func(t *testing.T, doc *Document) *TrashPlan {
		t.Helper()
		plan, err := doc.PrepareTrash()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { plan.Close() })
		return plan
	}
	refused := func(t *testing.T, plan *TrashPlan, label string) {
		t.Helper()
		called := false
		if err := plan.Apply(func(string) error { called = true; return nil }); !errors.Is(err, ErrTrashSourceChanged) || called {
			t.Fatalf("%s: %v, called=%v", label, err, called)
		}
	}

	t.Run("changed content", func(t *testing.T) {
		plan := prepare(t, open(t))
		if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
			t.Fatal(err)
		}
		refused(t, plan, "changed file moved")
	})
	t.Run("same content replacement", func(t *testing.T) {
		plan := prepare(t, open(t))
		source, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		retireTrashSource(t, path)
		if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, source.ModTime(), source.ModTime()); err != nil {
			t.Fatal(err)
		}
		refused(t, plan, "replacement moved")
	})
	t.Run("symlink", func(t *testing.T) {
		plan := prepare(t, open(t))
		backup := filepath.Join(root, "backup.txt")
		if err := os.WriteFile(backup, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
		retireTrashSource(t, path)
		if err := os.Symlink(backup, path); err != nil {
			t.Fatal(err)
		}
		refused(t, plan, "symlink moved")
	})
}

// TestTrashPlanRejectsSameContentReplacement exercises the whole
// PrepareTrash -> replace -> Apply path with a replacement that matches the
// captured file in content, size and modification time. None of those are
// identity, so only the descriptor-pinned device+inode comparison can tell the
// files apart; the backend move must not run.
func TestTrashPlanRejectsSameContentReplacement(t *testing.T) {
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
	defer plan.Close()
	source, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	retireTrashSource(t, path)
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, source.ModTime(), source.ModTime()); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "original" {
		t.Fatalf("replacement content = %q, %v", data, err)
	}
	if replacement.Size() != source.Size() || !replacement.ModTime().Equal(source.ModTime()) {
		t.Fatalf("replacement does not match captured size/time: %+v vs %+v", replacement, source)
	}
	if os.SameFile(source, replacement) {
		t.Fatal("replacement reused the inode held by the plan's open descriptor")
	}
	called := false
	if err := plan.Apply(func(string) error { called = true; return nil }); !errors.Is(err, ErrTrashSourceChanged) || called {
		t.Fatalf("same-content replacement moved: %v, called=%v", err, called)
	}
}

// TestTrashPlanReleasesSourceDescriptor pins the descriptor lifetime: Apply
// releases it on a successful move, a refusal and a backend error, Close
// releases a cancelled plan, and a consumed plan refuses a second move without
// touching the backend.
func TestTrashPlanReleasesSourceDescriptor(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	write := func(t *testing.T, data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	prepare := func(t *testing.T) (*Document, *TrashPlan) {
		t.Helper()
		doc, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := doc.PrepareTrash()
		if err != nil {
			t.Fatal(err)
		}
		return doc, plan
	}
	closed := func(t *testing.T, file *os.File, label string) {
		t.Helper()
		if file == nil {
			t.Fatalf("%s: prepared plan has no source descriptor", label)
		}
		if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("%s: source descriptor still open: %v", label, err)
		}
	}

	// Refusal releases the descriptor and makes the plan one-shot.
	write(t, "original")
	doc, plan := prepare(t)
	held := plan.file
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(func(string) error { return nil }); !errors.Is(err, ErrTrashSourceChanged) {
		t.Fatalf("refusal = %v", err)
	}
	closed(t, held, "refused apply")
	called := false
	if err := plan.Apply(func(string) error { called = true; return nil }); !errors.Is(err, ErrTrashSourceChanged) || called {
		t.Fatalf("consumed plan moved the source: %v, called=%v", err, called)
	}
	if err := plan.Close(); err != nil {
		t.Fatalf("close after apply = %v", err)
	}

	// A backend error releases it too.
	write(t, "original")
	doc, plan = prepare(t)
	held = plan.file
	failure := errors.New("trash unavailable")
	if err := plan.Apply(func(string) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("backend failure = %v", err)
	}
	closed(t, held, "backend failure")
	if doc.Dirty() {
		t.Fatal("backend failure dirtied the document")
	}

	// Close releases a plan the user cancelled, and is idempotent.
	write(t, "original")
	doc, plan = prepare(t)
	held = plan.file
	if err := plan.Close(); err != nil {
		t.Fatalf("cancel close = %v", err)
	}
	closed(t, held, "cancel")
	if err := plan.Close(); err != nil {
		t.Fatalf("second close = %v", err)
	}
	var nilPlan *TrashPlan
	if err := nilPlan.Close(); err != nil {
		t.Fatalf("nil plan close = %v", err)
	}
	if err := nilPlan.Apply(func(string) error { return nil }); !errors.Is(err, ErrTrashSourceChanged) {
		t.Fatalf("nil plan apply = %v", err)
	}
	_ = doc
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
	prepare := func() *TrashPlan {
		t.Helper()
		plan, err := doc.PrepareTrash()
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	failure := errors.New("trash unavailable")
	plan := prepare()
	if err := plan.Apply(func(string) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("backend failure = %v", err)
	}
	if plan.file != nil {
		t.Fatal("backend failure leaked the source descriptor")
	}
	plan = prepare()
	if err := plan.Apply(func(string) error { return nil }); !errors.Is(err, ErrTrashSourceRemains) {
		t.Fatalf("backend reported success without moving = %v", err)
	}
	plan = prepare()
	if err := plan.Apply(func(source string) error { return os.Rename(source, filepath.Join(root, "moved.txt")) }); err != nil {
		t.Fatal(err)
	}
	if !doc.Dirty() || doc.Editor.String() != "draft original" || doc.Path != canonicalPath {
		t.Fatalf("trash mutated document before caller closes it: path=%q dirty=%v value=%q", doc.Path, doc.Dirty(), doc.Editor.String())
	}
}
