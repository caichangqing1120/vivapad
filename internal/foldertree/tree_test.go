package foldertree

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCanonicalRootAndPathContainment(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "note.txt")
	if err := os.WriteFile(file, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalRoot(root)
	if err != nil || !filepath.IsAbs(canonical) || !Contains(canonical, file) || Contains(canonical, filepath.Join(parent, "project-copy", "note.txt")) {
		t.Fatalf("canonical=%q err=%v contains file=%v", canonical, err, Contains(canonical, file))
	}
	if _, err := CanonicalRoot(file); err == nil {
		t.Fatal("file was accepted as a folder root")
	}
	if runtime.GOOS != "windows" {
		alias := filepath.Join(parent, "alias")
		if err := os.Symlink(root, alias); err != nil {
			t.Fatal(err)
		}
		resolved, err := CanonicalRoot(alias)
		if err != nil || resolved != canonical {
			t.Fatalf("alias root = %q, %v; want %q", resolved, err, canonical)
		}
	}
}

func TestChildrenSortsDirectoriesBeforeFilesAndSkipsUnsafeLinks(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"beta", "Alpha", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"z.txt", "A.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(root, filepath.Join(root, "cycle")); err != nil {
			t.Fatal(err)
		}
	}
	children, err := Children(root)
	if err != nil || len(children) != 4 {
		t.Fatalf("children = %+v, %v", children, err)
	}
	for index, name := range []string{"Alpha", "beta", "A.txt", "z.txt"} {
		if children[index].Name != name || children[index].Directory != (index < 2) {
			t.Fatalf("children order = %+v", children)
		}
	}
}
