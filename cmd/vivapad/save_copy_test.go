package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
)

func TestSaveCopyAsKeepsActiveSavedDocumentDirty(t *testing.T) {
	root := t.TempDir()
	source, copyPath := filepath.Join(root, "source.txt"), filepath.Join(root, "copy.txt")
	if err := os.WriteFile(source, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{source})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	clickID(t, driver, editorID)
	if err := driver.Type("new"); err != nil {
		t.Fatal(err)
	}
	canonical := doc.Path
	if err := driver.QueueSaveFile(copyPath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(4))
	if data, err := os.ReadFile(copyPath); err != nil || string(data) != "oldnew" {
		t.Fatalf("saved copy = %q, %v", data, err)
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != "old" || state.space.Current() != doc || doc.Path != canonical || !doc.Dirty() {
		t.Fatalf("source changed after copy = %q, %v, doc=%+v", data, err, doc)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "old" {
		t.Fatalf("copy damaged undo = %q", doc.Editor.String())
	}
}

func TestSaveCopyAsRejectsOtherOpenPathEvenThroughAlias(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	if err := os.WriteFile(first, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{first, second})
	state.space.Select(0)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueSaveFile(filepath.Join(alias, "second.txt"), nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(4))
	if data, err := os.ReadFile(second); err != nil || string(data) != "second" || !strings.Contains(state.notice, "already open") {
		t.Fatalf("copy overwrote another tab: %q, %v, notice=%q", data, err, state.notice)
	}
}

func TestSaveCopyAsCanceledUntitledFileRemainsUntitled(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("draft"); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(4))
	if state.space.Current().Path != "" || !state.space.Current().Dirty() || state.space.Current().Editor.String() != "draft" {
		t.Fatalf("canceled copy changed untitled document: %+v", state.space.Current())
	}
	path := filepath.Join(t.TempDir(), "copy.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(4))
	if data, err := os.ReadFile(path); err != nil || string(data) != "draft" || state.space.Current().Path != "" || !state.space.Current().Dirty() {
		t.Fatalf("untitled copy = %q, %v, doc=%+v", data, err, state.space.Current())
	}
}
