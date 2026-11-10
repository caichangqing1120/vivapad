package document

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRenameMovesSavedDiskFileWithoutSavingDirtyBuffer(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	oldPath, newPath := filepath.Join(root, "old.txt"), filepath.Join(root, "new.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert(" new")
	plan, err := doc.PrepareRename(newPath)
	if err != nil || plan.TargetExists || plan.Target != newPath {
		t.Fatalf("rename plan = %+v, %v", plan, err)
	}
	if err := plan.Apply(false); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(newPath); err != nil || string(data) != "old" {
		t.Fatalf("moved disk file = %q, %v", data, err)
	}
	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists: %v", err)
	}
	if doc.Path != newPath || doc.Name != "new.txt" || !doc.Dirty() || doc.Editor.String() != "old new" {
		t.Fatalf("rename changed unsaved buffer: %+v", doc)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "old" || !doc.Editor.Redo() || doc.Editor.String() != "old new" {
		t.Fatalf("rename lost undo history: %q", doc.Editor.String())
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(newPath); err != nil || string(data) != "old new" || doc.Dirty() {
		t.Fatalf("save after rename = %q, %v, dirty=%v", data, err, doc.Dirty())
	}
}

func TestRenameRequiresExplicitOverwriteAndRejectsChangedDestination(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	oldPath, target := filepath.Join(root, "old.txt"), filepath.Join(root, "target.txt")
	for path, content := range map[string]string{oldPath: "source", target: "previous"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	doc, err := Open(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := doc.PrepareRename(target)
	if err != nil || !plan.TargetExists {
		t.Fatalf("existing destination plan = %+v, %v", plan, err)
	}
	if err := plan.Apply(false); !errors.Is(err, ErrRenameConfirmationRequired) {
		t.Fatalf("unconfirmed overwrite = %v", err)
	}
	if err := os.WriteFile(target, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(true); !errors.Is(err, ErrRenameTargetChanged) {
		t.Fatalf("stale destination accepted: %v", err)
	}
	if data, err := os.ReadFile(oldPath); err != nil || string(data) != "source" || doc.Path != oldPath {
		t.Fatalf("failed rename changed source: %q, %v, path=%q", data, err, doc.Path)
	}
	plan, err = doc.PrepareRename(target)
	if err != nil || !plan.TargetExists {
		t.Fatalf("fresh plan = %+v, %v", plan, err)
	}
	if err := plan.Apply(true); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "source" || doc.Path != target {
		t.Fatalf("confirmed replacement = %q, %v, path=%q", data, err, doc.Path)
	}
}

func TestRenameRejectsChangedSourceAndNewDestinationAppearingAfterPreview(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	oldPath, target := filepath.Join(root, "old.txt"), filepath.Join(root, "target.txt")
	if err := os.WriteFile(oldPath, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := doc.PrepareRename(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("unexpected"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(true); !errors.Is(err, ErrRenameTargetChanged) {
		t.Fatalf("new destination appeared without confirmation: %v", err)
	}
	if err := os.WriteFile(oldPath, []byte("external edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(false); !errors.Is(err, ErrRenameSourceChanged) {
		t.Fatalf("stale source accepted: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "unexpected" || doc.Path != oldPath {
		t.Fatalf("stale rename changed destination or binding: %q, %v, path=%q", data, err, doc.Path)
	}
}
