package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestPathCompletionMenuQuotesAndUndo(t *testing.T) {
	root := t.TempDir()
	canonical, _ := filepath.EvalSymlinks(root)
	for _, name := range []string{"alpha.txt", "alphabet.txt"} {
		if err := os.WriteFile(filepath.Join(canonical, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	prefix := "open \"" + filepath.Join(canonical, "alp")
	source := filepath.Join(canonical, "source.txt")
	if err := os.WriteFile(source, []byte(prefix), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{source})
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(18))
	clickID(t, driver, autoCompletionScope.At(5))
	if current := state.wordCompletion; current == nil || current.kind != completionPath || len(current.items) != 2 {
		t.Fatalf("path menu popup = %+v, notice=%q", current, state.notice)
	}
	clickID(t, driver, wordCompletionScope.At(0))
	want := "open \"" + filepath.Join(canonical, "alpha.txt")
	if doc.Editor.String() != want || !doc.Dirty() || state.wordCompletion != nil {
		t.Fatalf("accepted path = %q, popup=%+v", doc.Editor.String(), state.wordCompletion)
	}
	state.command("Undo")
	if doc.Editor.String() != prefix {
		t.Fatalf("path completion undo = %q", doc.Editor.String())
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != prefix {
		t.Fatalf("path completion wrote disk: %q, %v", data, err)
	}
}

func TestPathCompletionShortcutFiltersAndAcceptsWithNativeEnter(t *testing.T) {
	root := t.TempDir()
	canonical, _ := filepath.EvalSymlinks(root)
	src := filepath.Join(canonical, "src")
	if err := os.Mkdir(src, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.go", "manual.go"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(canonical)
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.space.Current().Editor.ClearHistory()
	clickID(t, driver, editorID)
	if err := driver.Type("src/ma"); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, ' '); err != nil || !handled {
		t.Fatalf("path shortcut handled=%v err=%v", handled, err)
	}
	if current := state.wordCompletion; current == nil || current.kind != completionPath || len(current.items) != 2 {
		t.Fatalf("path shortcut popup = %+v, notice=%q", current, state.notice)
	}
	if err := driver.Type("i"); err != nil {
		t.Fatal(err)
	}
	if current := state.wordCompletion; current == nil || len(current.items) != 1 || current.items[0] != "src/main.go" {
		t.Fatalf("path filtering = %+v", current)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if got := state.space.Current().Editor.String(); got != "src/main.go" || state.wordCompletion != nil {
		t.Fatalf("native path acceptance = %q, popup=%+v", got, state.wordCompletion)
	}
}

func TestPathCompletionClearsPreviousNoMatchWhenPopupOpens(t *testing.T) {
	root := t.TempDir()
	const before = "missing\n[al-old.txt]"
	source := filepath.Join(root, "buffer.txt")
	for name, contents := range map[string]string{"buffer.txt": before, "alpha.txt": "target"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{source})
	doc := state.space.Current()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	completeAt := func(offset int) {
		t.Helper()
		doc.Editor.MoveTo(offset, false)
		if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, ' '); err != nil || !handled {
			t.Fatalf("path shortcut handled=%v err=%v", handled, err)
		}
	}
	hasStatus := func() bool {
		for _, label := range driver.Snapshot().Texts {
			if label.Value == "No path completion" {
				return true
			}
		}
		return false
	}
	completeAt(len("missing"))
	if state.wordCompletion != nil || !hasStatus() {
		t.Fatalf("missing-path status did not appear: %q", state.notice)
	}
	completeAt(len("missing\n[al"))
	if current := state.wordCompletion; current == nil || len(current.items) != 1 || current.items[0] != "alpha.txt" {
		t.Fatalf("expected filename popup, got %+v", current)
	}
	if hasStatus() {
		t.Fatalf("stale status remains visible beside the path popup: %q", state.notice)
	}
	if doc.Editor.String() != before || doc.Dirty() {
		t.Fatal("opening the candidate popup modified the document")
	}
}

func TestPathCompletionDirectoryReadonlyAndMissingParent(t *testing.T) {
	root := t.TempDir()
	canonical, _ := filepath.EvalSymlinks(root)
	if err := os.Mkdir(filepath.Join(canonical, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(canonical)
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("./s")
	// A directory candidate gains the platform's path separator, so the
	// expectation is "./src/" on macOS and Linux and "./src\\" on Windows.
	wanted := "./src" + string(filepath.Separator)
	state.command("Path Completion")
	if current := state.wordCompletion; current == nil || len(current.items) != 1 || current.items[0] != wanted {
		t.Fatalf("directory suffix popup = %+v, notice=%q", current, state.notice)
	}
	state.acceptWordCompletion(0)
	if got := doc.Editor.String(); got != wanted {
		t.Fatalf("directory path insertion = %q, want %q", got, wanted)
	}
	state.command("Read-Only on Current Document")
	state.command("Path Completion")
	if state.wordCompletion != nil || !strings.Contains(state.notice, "Read-only") {
		t.Fatal("read-only path completion opened")
	}
	state.command("Clear Read-Only for All Documents")
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	doc.Editor.Insert("missing/ma")
	state.command("Path Completion")
	if state.wordCompletion != nil || !strings.Contains(state.notice, "missing") {
		t.Fatalf("missing parent did not report error: %q", state.notice)
	}
}

func TestPathCompletionRespectsActiveSplitPane(t *testing.T) {
	root := t.TempDir()
	canonical, _ := filepath.EvalSymlinks(root)
	if err := os.WriteFile(filepath.Join(canonical, "alpha.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	left := state.space.Current()
	right := state.space.NewDocument()
	right.Editor.Insert(filepath.Join(canonical, "al"))
	state.toggleSplit()
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	right.Editor.MoveTo(right.Editor.RuneCount(), false)
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, ' '); err != nil || !handled {
		t.Fatalf("split shortcut handled=%v err=%v", handled, err)
	}
	if current := state.wordCompletion; current == nil || current.doc != right || current.pane != 1 || left.Editor.String() != "" {
		t.Fatalf("split path completion = %+v, left=%q", current, left.Editor.String())
	}
}

func TestPathCompletionUsesSavedFileDirectoryAndRejectsActiveIME(t *testing.T) {
	root := t.TempDir()
	canonical, _ := filepath.EvalSymlinks(root)
	for _, name := range []string{"src", "notes"} {
		if err := os.Mkdir(filepath.Join(canonical, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(canonical, "src", "main.go"), []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(canonical, "notes", "current.txt")
	if err := os.WriteFile(source, []byte("../src/ma"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{source})
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	state.command("Path Completion")
	if current := state.wordCompletion; current == nil || len(current.items) != 1 || current.items[0] != "../src/main.go" {
		t.Fatalf("saved-file relative base = %+v, notice=%q", current, state.notice)
	}
	state.wordCompletion = nil
	doc.Editor.SetMarkedText("x", text.Range{Start: 1, End: 1})
	before := doc.Editor.String()
	state.command("Path Completion")
	if state.wordCompletion != nil || doc.Editor.String() != before {
		t.Fatal("path completion interrupted active IME composition")
	}
}

func TestPathCompletionPopupDistinguishesLongCandidates(t *testing.T) {
	root := t.TempDir()
	canonical, _ := filepath.EvalSymlinks(root)
	for _, name := range []string{"go.mod", "go.sum"} {
		if err := os.WriteFile(filepath.Join(canonical, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.space.Current().Editor.Insert(filepath.Join(canonical, "go."))
	state.command("Path Completion")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, label := range driver.Snapshot().Texts {
		if label.Value == "go.mod" || label.Value == "go.sum" {
			found[label.Value] = true
		}
	}
	if !found["go.mod"] || !found["go.sum"] {
		t.Fatalf("long path options are indistinguishable: %+v", driver.Snapshot().Texts)
	}
}

func TestPathCompletionReportsTruncationWithoutVisibleCandidates(t *testing.T) {
	root := t.TempDir()
	for index := range 2001 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("other-%04d.txt", index)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	canonical, _ := filepath.EvalSymlinks(root)
	state := newModel()
	state.space.Current().Editor.Insert(filepath.Join(canonical, "wanted"))
	state.command("Path Completion")
	if state.wordCompletion != nil || !strings.Contains(state.notice, "first 2000") {
		t.Fatalf("truncated empty candidate report = %+v, %q", state.wordCompletion, state.notice)
	}
}
