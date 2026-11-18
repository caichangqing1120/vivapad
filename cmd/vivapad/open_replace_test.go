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

func TestReplaceAllOpenDocumentsSkipsEqualTabsThroughSearchMenu(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{name: "desktop", width: 1280, height: 800},
		{name: "compact", width: 640, height: 400},
	} {
		t.Run(size.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "unchanged.txt")
			if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{path})
			unchanged := state.space.Current()
			state.readOnly[unchanged] = true
			changed := state.space.NewDocument()
			changed.Editor.Insert("cat dog")
			changed.Editor.SetSavePoint()
			state.showInActivePane(changed)
			state.command("Clone to Other View")
			if state.paneDocument(0) != changed || state.paneDocument(1) != changed {
				t.Fatal("fixture did not establish a shared document in both panes")
			}
			state.activePane = 1
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			if err := driver.Resize(size.width, size.height); err != nil {
				t.Fatal(err)
			}
			state.query.Insert(`(cat|dog)`)
			state.replacement.Insert("cat")
			state.searchOptions.Regex = true
			clickID(t, driver, menuScope.At(2))
			revealSearchMenuItem(t, driver, searchCommandIndex(t, "Replace All in Open Documents"))
			clickID(t, driver, searchCommandID(t, "Replace All in Open Documents"))
			if unchanged.Editor.String() != "cat" || unchanged.Dirty() || unchanged.Editor.CanUndo() ||
				changed.Editor.String() != "cat cat" || !changed.Dirty() ||
				state.paneDocument(0) != changed || state.paneDocument(1) != changed ||
				state.notice != "Unsaved: 2 matches replaced in 1 open tabs" {
				t.Fatalf("mixed command changed untouched tab or lost clone: unchanged=%q changed=%q notice=%q", unchanged.Editor.String(), changed.Editor.String(), state.notice)
			}
			if !changed.Editor.Undo() || changed.Editor.String() != "cat dog" ||
				!changed.Editor.Redo() || changed.Editor.String() != "cat cat" {
				t.Fatalf("changed clone did not keep one Undo unit: %q", changed.Editor.String())
			}
			if disk, err := os.ReadFile(path); err != nil || string(disk) != "cat" {
				t.Fatalf("command wrote skipped disk: %q, %v", disk, err)
			}
		})
	}
}

func TestReplaceAllOpenDocumentsReportsNoChangeForEqualWholeTab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unchanged.txt")
	if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.readOnly[doc] = true
	state.query.Insert("cat")
	state.replacement.Insert("cat")
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	revealSearchMenuItem(t, driver, searchCommandIndex(t, "Replace All in Open Documents"))
	clickID(t, driver, searchCommandID(t, "Replace All in Open Documents"))
	if state.notice != "No open-document replacements needed" || doc.Editor.String() != "cat" || doc.Dirty() || doc.Editor.CanUndo() {
		t.Fatalf("all-equal command changed saved tab: text=%q dirty=%t notice=%q", doc.Editor.String(), doc.Dirty(), state.notice)
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != "cat" {
		t.Fatalf("all-equal command wrote disk: %q, %v", disk, err)
	}
}

func TestReplaceAllOpenDocumentsRefusesChangedReadOnlyTabAtBothSizes(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{name: "desktop", width: 1280, height: 800},
		{name: "compact", width: 640, height: 400},
	} {
		t.Run(size.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "unchanged.txt")
			if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{path})
			unchanged := state.space.Current()
			changed := state.space.NewDocument()
			changed.Editor.Insert("cat dog")
			state.showInActivePane(changed)
			state.readOnly[changed] = true
			beforeDirty, beforeUndo := changed.Dirty(), changed.Editor.CanUndo()
			state.query.Insert(`(cat|dog)`)
			state.replacement.Insert("cat")
			state.searchOptions.Regex = true
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			if err := driver.Resize(size.width, size.height); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(2))
			revealSearchMenuItem(t, driver, searchCommandIndex(t, "Replace All in Open Documents"))
			clickID(t, driver, searchCommandID(t, "Replace All in Open Documents"))
			if state.notice != "Read-only document in replacement plan" ||
				unchanged.Editor.String() != "cat" || unchanged.Dirty() || unchanged.Editor.CanUndo() ||
				changed.Editor.String() != "cat dog" || changed.Dirty() != beforeDirty || changed.Editor.CanUndo() != beforeUndo {
				t.Fatalf("refused changed target mutated a tab: unchanged=%q changed=%q notice=%q", unchanged.Editor.String(), changed.Editor.String(), state.notice)
			}
			if disk, err := os.ReadFile(path); err != nil || string(disk) != "cat" {
				t.Fatalf("refused changed target wrote disk: %q, %v", disk, err)
			}
		})
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
	clickID(t, driver, searchCommandID(t, "Replace All in Open Documents"))
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
	clickID(t, driver, searchCommandID(t, "Find All in Open Documents"))
	if !state.resultsOpen || !state.resultOpenDocuments {
		t.Fatal("missing prior search results")
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Replace All in Open Documents"))
	if state.resultsOpen || state.resultOpenDocuments || len(state.resultDocuments) != 0 || state.space.Current().Editor.String() != "dog dog" {
		t.Fatalf("replace left stale results: open=%v sources=%d text=%q", state.resultsOpen, len(state.resultDocuments), state.space.Current().Editor.String())
	}
}

func TestCompactOpenReplacementShowsMixedEOLRefusalReason(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed-line-endings-document.txt")
	if err := os.WriteFile(path, []byte("first\rmiddle\nlast"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.query.Insert("middle")
	clickID(t, driver, menuScope.At(2))
	revealSearchMenuItem(t, driver, searchCommandIndex(t, "Replace All in Open Documents"))
	clickID(t, driver, searchCommandID(t, "Replace All in Open Documents"))
	for _, label := range driver.Snapshot().Texts {
		if strings.Contains(label.Value, "adjacent CR and LF would merge") {
			return
		}
	}
	t.Fatalf("compact status hid the preflight reason: %q", state.notice)
}
