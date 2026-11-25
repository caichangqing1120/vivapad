package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestSelectedPathCommandsResolveSavedRelativeAndKeepDirtyTab(t *testing.T) {
	root := t.TempDir()
	current, target := filepath.Join(root, "current.txt"), filepath.Join(root, "sub", "a name.txt")
	if err := os.Mkdir(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte("initial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{current})
	doc := state.space.Current()
	doc.Editor.Insert(` "sub/a name.txt"`)
	value := doc.Editor.String()
	start := strings.Index(value, `"sub/a name.txt"`)
	doc.Editor.SetSelection(text.Range{Start: len([]rune(value[:start])), End: len([]rune(value[:start+len(`"sub/a name.txt"`)]))})
	var opened, revealed string
	state.launchSelectedFile = func(path string) error { opened = path; return nil }
	state.revealSelectedPath = func(path string) error { revealed = path; return nil }
	state.command("Open File from Selection")
	state.command("Open Containing Folder from Selection")
	canonicalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if opened != canonicalTarget || revealed != canonicalTarget || !doc.Dirty() || doc.Editor.String() != value || len(state.space.Documents) != 1 {
		t.Fatalf("selected path actions: open=%q reveal=%q text=%q dirty=%v", opened, revealed, doc.Editor.String(), doc.Dirty())
	}
}

func TestSelectedPathAtCaretAndEnvironmentExpansion(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert(`open "%VIVAPAD_SELECTED_ROOT%/notes.txt" later`)
	doc.Editor.MoveTo(len([]rune(`open "%VIVAPAD_SELECTED_ROOT%/note`)), false)
	t.Setenv("VIVAPAD_SELECTED_ROOT", root)
	var got string
	state.launchSelectedFile = func(path string) error { got = path; return nil }
	state.command("Open File from Selection")
	if got != file || doc.Editor.String() != `open "%VIVAPAD_SELECTED_ROOT%/notes.txt" later` {
		t.Fatalf("caret path = %q, text=%q, notice=%q", got, doc.Editor.String(), state.notice)
	}
}

func TestSelectedPathEnvironmentValueDoesNotExpandAgain(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "cost$tag")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "notes.txt")
	if err := os.WriteFile(file, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIVAPAD_LITERAL_ROOT", folder)
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("%VIVAPAD_LITERAL_ROOT%/notes.txt")
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	var launched string
	state.launchSelectedFile = func(path string) error { launched = path; return nil }
	state.command("Open File from Selection")
	if launched != file {
		t.Fatalf("environment value expanded a second time: path=%q notice=%q", launched, state.notice)
	}
}

func TestSelectedPathHomeAndUntitledWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Chdir(root)
	file := filepath.Join(root, "note with spaces.txt")
	if err := os.WriteFile(file, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	doc := state.space.Current()
	var opened []string
	state.launchSelectedFile = func(path string) error { opened = append(opened, path); return nil }
	for _, path := range []string{`"~/note with spaces.txt"`, `"./note with spaces.txt"`} {
		doc.Editor.ReplaceRange(text.Range{Start: 0, End: doc.Editor.RuneCount()}, path)
		doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
		state.command("Open File from Selection")
	}
	if len(opened) != 2 || opened[0] != file || opened[1] != file || doc.Path != "" {
		t.Fatalf("untitled selected paths = %v, notice=%q", opened, state.notice)
	}
}

func TestSelectedPathRevealDirectoryPreservesSplitViews(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current.txt")
	if err := os.WriteFile(current, []byte("initial"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{current})
	state.command("Clone to Other View")
	state.activePane = 1
	doc := state.paneDocument(1)
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("\nsubfolder")
	if err := os.Mkdir(filepath.Join(root, "subfolder"), 0700); err != nil {
		t.Fatal(err)
	}
	value := doc.Editor.String()
	start := strings.Index(value, "subfolder")
	doc.Editor.SetSelection(text.Range{Start: len([]rune(value[:start])), End: doc.Editor.RuneCount()})
	var revealed string
	state.revealSelectedPath = func(path string) error { revealed = path; return nil }
	state.command("Open Containing Folder from Selection")
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if revealed != filepath.Join(canonicalRoot, "subfolder") || !doc.Dirty() || doc.Editor.String() != value ||
		state.activePane != 1 || state.paneDocument(0) != doc || state.paneDocument(1) != doc {
		t.Fatalf("split directory reveal = %q, active=%d, dirty=%v, text=%q, notice=%q", revealed, state.activePane, doc.Dirty(), doc.Editor.String(), state.notice)
	}
}

func TestSelectedPathRefusesMissingUnsafeAndMultipleSelections(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "target.txt")
	if err := os.WriteFile(file, []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert(file)
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	called := false
	state.launchSelectedFile = func(string) error { called = true; return nil }
	state.revealSelectedPath = func(string) error { called = true; return nil }
	state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), ranges: []text.Range{{Start: 0, End: 2}, {Start: 2, End: 4}}, active: true}
	doc.Editor.SetSelection(text.Range{Start: 2, End: 4})
	state.command("Open File from Selection")
	state.command("Open Containing Folder from Selection")
	if called || !strings.Contains(state.notice, "single") {
		t.Fatalf("multi-selection launched path: called=%v notice=%q", called, state.notice)
	}
	state.multiSelections[0] = multiSelectionState{}
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: doc.Editor.RuneCount()}, file+"\x00")
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	state.command("Open File from Selection")
	if called {
		t.Fatal("unsafe path launched a process")
	}
}

func TestSelectedPathMenuActionsReachableAtCompactSize(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert(file)
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	var launched, revealed string
	state.launchSelectedFile = func(path string) error { launched = path; return nil }
	state.revealSelectedPath = func(path string) error { revealed = path; return nil }
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	choose := func(index int) {
		clickID(t, driver, menuScope.At(1))
		clickEditMenuCommand(t, driver, "On Selection")
		for row := range 3 {
			found := false
			for _, hit := range driver.Snapshot().Hits {
				if hit.ID == onSelectionScope.At(row) {
					found = true
					if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
						t.Fatalf("selected-path row %d clipped: %+v", row, hit.Bounds)
					}
				}
			}
			if !found {
				t.Fatalf("selected-path row %d missing", row)
			}
		}
		clickID(t, driver, onSelectionScope.At(index))
	}
	choose(0)
	choose(1)
	if launched != file || revealed != file || doc.Editor.String() != file || len(state.space.Documents) != 1 {
		t.Fatalf("menu actions = launched:%q revealed:%q text:%q", launched, revealed, doc.Editor.String())
	}
}

func TestCaptureSelectedPathMenuWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_SELECTED_PATH_SHOT")
	if output == "" {
		t.Skip("set VIVAPAD_SELECTED_PATH_SHOT to capture On Selection menu")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("open ./notes.txt")
	doc.Editor.SetSelection(text.Range{Start: 5, End: doc.Editor.RuneCount()})
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), menuScope.At(1))
	clickEditMenuCommand(t, session.Driver(), "On Selection")
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("On Selection menu layout: %+v", problems)
	}
	if err := session.Screenshot(output); err != nil {
		t.Fatal(err)
	}
}

func TestSelectedPathFailureNeverLaunchesOrChangesDocument(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "target.txt")
	if err := os.WriteFile(file, []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert(file)
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	state.launchSelectedFile = func(string) error { return errors.New("launch denied") }
	state.command("Open File from Selection")
	if doc.Editor.String() != file || !strings.Contains(state.notice, "launch denied") || len(state.space.Documents) != 1 {
		t.Fatalf("launch failure changed tab: %q, notice=%q", doc.Editor.String(), state.notice)
	}
	called := false
	state.launchSelectedFile = func(string) error { called = true; return nil }
	state.revealSelectedPath = func(string) error { called = true; return nil }
	for _, target := range []string{root, filepath.Join(root, "missing.txt"), "%VIVAPAD_UNKNOWN_SELECTED_PATH%/target.txt"} {
		doc.Editor.ReplaceRange(text.Range{Start: 0, End: doc.Editor.RuneCount()}, target)
		doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
		state.command("Open File from Selection")
		if called {
			t.Fatalf("unsafe selected file launched: %q", target)
		}
	}
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: doc.Editor.RuneCount()}, file)
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	state.rectangles[0] = rectangleSelection{doc: doc, value: doc.Editor.String(), active: true}
	state.command("Open Containing Folder from Selection")
	if called || !strings.Contains(state.notice, "single") {
		t.Fatalf("rectangular path launched: called=%v notice=%q", called, state.notice)
	}
}
