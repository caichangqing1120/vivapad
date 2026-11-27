package projects

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectReferenceLifecycle(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "projects.json")
	file := filepath.Join(root, "kept.txt")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	groups := []Group{{Name: "Demo", Files: []string{file}}}
	if err := Save(path, groups); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, append(groups, Group{Name: "demo"})); err == nil {
		t.Fatal("accepted duplicate name")
	}
	groups, err := Load(path)
	if err != nil || len(groups) != 1 || groups[0].Files[0] != file {
		t.Fatalf("load = %+v, %v", groups, err)
	}
	if err := Save(path, nil); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(file); err != nil || string(content) != "keep" {
		t.Fatalf("deleting group affected its file: %q, %v", content, err)
	}
}
