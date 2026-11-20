package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestPathCompletionAllocatesForCaretLineAndPreservesUnicodeOffsets(t *testing.T) {
	root := t.TempDir()
	const target = "alpha.txt"
	if err := os.WriteFile(filepath.Join(root, target), []byte("candidate"), 0600); err != nil {
		t.Fatal(err)
	}
	background := strings.Repeat("unrelated \u732b\n", 1<<16)
	const left, typed, right = "open \u732b \"", "al-old.txt", "\" keep\n"
	before := background + left + typed + right + background
	path := filepath.Join(root, "buffer.txt")
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	start := doc.Editor.RuneIndex(len(background) + len(left))
	doc.Editor.MoveTo(start+len("al"), false)
	if doc.Editor.String() != before {
		t.Fatal("source was not opened as expected")
	}
	var initial, final runtime.MemStats
	const lookups = 3
	runtime.ReadMemStats(&initial)
	for range lookups {
		state.startPathCompletion()
	}
	runtime.ReadMemStats(&final)
	bytesPerLookup := (final.TotalAlloc - initial.TotalAlloc) / lookups
	t.Logf("%d source bytes, %d allocated bytes per path lookup", len(before), bytesPerLookup)
	if bytesPerLookup > 512<<10 {
		t.Errorf("path completion allocated %d bytes per lookup for a short caret line", bytesPerLookup)
	}
	current := state.wordCompletion
	if current == nil || current.start != start || current.end != start+len(typed) || len(current.items) != 1 || current.items[0] != target {
		t.Fatalf("path popup had the wrong range or candidate, notice=%q", state.notice)
	}
	state.acceptWordCompletion(0)
	if doc.Editor.String() != background+left+target+right+background {
		t.Fatal("completion changed the quoted range or surrounding text")
	}
	state.command("Undo")
	if doc.Editor.String() != before {
		t.Fatal("completion did not undo in a single step")
	}
	if saved, err := os.ReadFile(path); err != nil || string(saved) != before {
		t.Fatalf("completion changed the saved file: %v", err)
	}
}

func TestPathCompletionMenuQuotesAndUndo(t *testing.T) {
	root := t.TempDir()
	canonical, _ := filepath.EvalSymlinks(root)
	names := []string{"alpha.txt", "alphabet.txt"}
	if runtime.GOOS != "windows" {
		names = append(names, `alpha"draft.txt`, "alpha\nnext.txt")
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(canonical, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	const left = `it's ready: "`
	prefix, suffix := left+filepath.Join(canonical, "alp"), "\" keep"
	before := prefix + suffix
	source := filepath.Join(canonical, "source.txt")
	if err := os.WriteFile(source, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{source})
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneIndex(len(prefix)), false)
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
	want := left + filepath.Join(canonical, "alpha.txt") + suffix
	if doc.Editor.String() != want || !doc.Dirty() || state.wordCompletion != nil {
		t.Fatalf("accepted path = %q, popup=%+v", doc.Editor.String(), state.wordCompletion)
	}
	state.command("Undo")
	if doc.Editor.String() != before {
		t.Fatalf("path completion undo = %q", doc.Editor.String())
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != before {
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

func TestPathCompletionRejectsDirectorySuggestionReplacedByFile(t *testing.T) {
	root := t.TempDir()
	const before = `open "./al-old" tail`
	const caret = len(`open "./al`)
	source := filepath.Join(root, "buffer.txt")
	if err := os.WriteFile(source, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(root, "alpha")
	if err := os.Mkdir(candidate, 0700); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{source})
	doc := state.space.Current()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(caret, false)

	openSuggestions := func(want string) {
		t.Helper()
		if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, ' '); err != nil || !handled {
			t.Fatalf("path shortcut handled=%v err=%v", handled, err)
		}
		if popup := state.wordCompletion; popup == nil || len(popup.items) != 1 || popup.items[0] != want {
			t.Fatalf("expected suggestion %q, got %+v (notice %q)", want, popup, state.notice)
		}
	}
	openSuggestions("./alpha" + string(filepath.Separator))
	if err := os.Remove(candidate); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidate, []byte("external file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != before || doc.Editor.Caret() != caret || doc.Dirty() || doc.Editor.CanUndo() ||
		state.wordCompletion != nil || state.notice != "Path suggestion is no longer available" {
		t.Fatalf("stale directory suggestion changed the editor: text=%q, caret=%d, notice=%q", doc.Editor.String(), doc.Editor.Caret(), state.notice)
	}

	openSuggestions("./alpha")
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != `open "./alpha" tail` {
		t.Fatalf("fresh file suggestion produced %q", got)
	}
	state.command("Undo")
	if doc.Editor.String() != before || doc.Dirty() || doc.Editor.CanUndo() {
		t.Fatalf("one Undo did not restore the saved text: %q", doc.Editor.String())
	}
	for name, want := range map[string]string{source: before, candidate: "external file"} {
		if data, err := os.ReadFile(name); err != nil || string(data) != want {
			t.Fatalf("completion changed disk file %q: %q, %v", name, data, err)
		}
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
