package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/document"
)

func TestReplaceAllOpenDocumentsChangesBuffersNotDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.txt")
	if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	first := state.space.Current()
	clickID(t, driver, newID)
	clickID(t, driver, editorID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	second := state.space.Current()
	state.query.Insert("cat")
	state.replacement.Insert("dog")
	clickID(t, driver, searchID)
	clickID(t, driver, replaceOpenID)
	if first.Editor.String() != "dog" || second.Editor.String() != "dog" || !first.Dirty() || !second.Dirty() ||
		state.space.Current() != second || !strings.HasPrefix(state.notice, "Unsaved:") || !strings.Contains(state.notice, "2 matches") {
		t.Fatalf("cross-tab replace = first:%q second:%q notice:%q", first.Editor.String(), second.Editor.String(), state.notice)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "cat" {
		t.Fatalf("cross-tab replace wrote disk: %q, %v", data, err)
	}
	if !first.Editor.Undo() || first.Editor.String() != "cat" || !second.Editor.Undo() || second.Editor.String() != "cat" {
		t.Fatalf("per-tab undo = %q, %q", first.Editor.String(), second.Editor.String())
	}
}

func TestReplaceAllOpenDocumentsFailsBeforeAnyTabWhenLegacyCannotEncode(t *testing.T) {
	root := t.TempDir()
	utfPath, legacyPath := filepath.Join(root, "utf.txt"), filepath.Join(root, "legacy.txt")
	if err := os.WriteFile(utfPath, []byte("cat"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("cat"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{utfPath})
	first := state.space.Current()
	if err := state.space.OpenWithEncoding(legacyPath, document.Windows1252); err != nil {
		t.Fatal(err)
	}
	second := state.space.Current()
	state.showInActivePane(second)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.query.Insert("cat")
	state.replacement.Insert("中文")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(9))
	if first.Editor.String() != "cat" || second.Editor.String() != "cat" || first.Dirty() || second.Dirty() ||
		!strings.Contains(state.notice, "cannot be saved") {
		t.Fatalf("unsafe batch partially changed tabs: first:%q second:%q notice:%q", first.Editor.String(), second.Editor.String(), state.notice)
	}
}

func TestReplaceAllOpenDocumentsInvalidatesOldFindResults(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("cat cat"); err != nil {
		t.Fatal(err)
	}
	state.query.Insert("cat")
	state.replacement.Insert("dog")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(8))
	if !state.resultsOpen || !state.resultOpenDocuments {
		t.Fatal("missing prior search results")
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(9))
	if state.resultsOpen || state.resultOpenDocuments || len(state.resultDocuments) != 0 || state.space.Current().Editor.String() != "dog dog" {
		t.Fatalf("replace left stale results: open=%v sources=%d text=%q", state.resultsOpen, len(state.resultDocuments), state.space.Current().Editor.String())
	}
}
