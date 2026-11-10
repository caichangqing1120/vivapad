package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/macro"
	"vivapad/internal/searchfiles"
)

func TestReadOnlyBlocksNativeToolbarMenuAndMacroEditsButAllowsCopy(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("cat cat"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.command("Read-Only on Current Document")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	readOnlyHit := false
	for _, hit := range driver.Snapshot().Hits {
		readOnlyHit = readOnlyHit || hit.ID == editorID && hit.ReadOnly
	}
	if !readOnlyHit || !state.isReadOnly(doc) || !state.menuSelected("Read-Only on Current Document") {
		t.Fatal("editor did not expose internal read-only state")
	}
	_ = driver.Type("X")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	driver.SetClipboard("prior")
	for _, command := range []string{"Cut", "Paste", "Delete", "Undo", "Redo", "Duplicate Line", "Delete Line", "Indent Lines", "Unindent Lines", "UPPERCASE", "Sort Lines Descending", "Trim Leading Space"} {
		state.command(command)
		if doc.Editor.String() != "cat cat" || driver.Clipboard() != "prior" {
			t.Fatalf("read-only command %q edited text/clipboard: %q, %q", command, doc.Editor.String(), driver.Clipboard())
		}
	}
	state.macroLast = []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "X"}}
	state.command("Playback Macro")
	if doc.Editor.String() != "cat cat" || !strings.Contains(state.notice, "Read-only") {
		t.Fatalf("macro changed read-only tab: %q, notice=%q", doc.Editor.String(), state.notice)
	}
	if _, err := driver.ShortcutChord(primary(), 'd'); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.ShortcutChord(primary()|ui.ModifierShift, 'k'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "cat cat" {
		t.Fatalf("shortcuts changed read-only text: %q", doc.Editor.String())
	}
	state.command("Copy")
	if driver.Clipboard() != "cat" || doc.Editor.String() != "cat cat" {
		t.Fatalf("copy was blocked or edited text: %q, %q", driver.Clipboard(), doc.Editor.String())
	}
}

func TestReadOnlyBlocksFindReplaceAndOpenDocumentBatchAtomically(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Editor.Insert("cat")
	state.command("Read-Only on Current Document")
	second := state.space.NewDocument()
	second.Editor.Insert("cat")
	state.showInActivePane(second)
	state.query.Insert("cat")
	state.replacement.Insert("dog")
	state.space.Select(0)
	state.showInActivePane(first)
	clickID(t, driver, searchID)
	clickID(t, driver, replaceID)
	clickID(t, driver, replaceAllID)
	if first.Editor.String() != "cat" {
		t.Fatal("find bar edited read-only document")
	}
	state.replaceAllOpenDocuments()
	if first.Editor.String() != "cat" || second.Editor.String() != "cat" || !strings.Contains(state.notice, "Read-only") {
		t.Fatalf("cross-tab replace partially changed buffers: first=%q second=%q notice=%q", first.Editor.String(), second.Editor.String(), state.notice)
	}
	state.command("Read-Only on Current Document")
	state.replaceAllOpenDocuments()
	if first.Editor.String() != "dog" || second.Editor.String() != "dog" {
		t.Fatalf("cleared read-only did not restore replacement: first=%q second=%q", first.Editor.String(), second.Editor.String())
	}
}

func TestReadOnlyAllAppliesOnlyToOpenDocumentsAndSharedClone(t *testing.T) {
	state := newModel()
	first := state.space.Current()
	second := state.space.NewDocument()
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{first, first}
	state.paneOwner[first], state.paneOwner[second] = 0, 1
	state.space.Select(0)
	state.command("Read-Only for All Documents")
	if !state.isReadOnly(first) || !state.isReadOnly(second) {
		t.Fatal("all-documents mode did not cover both existing tabs")
	}
	newDoc := state.space.NewDocument()
	if state.isReadOnly(newDoc) {
		t.Fatal("new tab inherited one-time read-only command")
	}
	state.command("Clear Read-Only for All Documents")
	if state.isReadOnly(first) || state.isReadOnly(second) || state.isReadOnly(newDoc) {
		t.Fatal("clear-all left a read-only document")
	}
}

func TestReadOnlySubmenuFitsCompactViewportAndTogglesCurrent(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(16))
	for index := range readOnlyCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == readOnlyScope.At(index) {
				found = true
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("read-only row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("read-only row %d missing", index)
		}
	}
	clickID(t, driver, readOnlyScope.At(0))
	if !state.isReadOnly(state.space.Current()) || state.readOnlyMenuOpen {
		t.Fatal("menu did not toggle current document")
	}
}

func TestReadOnlyStillAllowsCaretSelectionAndCopy(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abc")
	state.command("Read-Only on Current Document")
	doc.Editor.MoveTo(0, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if doc.Editor.Caret() != 3 {
		t.Fatalf("mouse cannot place read-only caret: %d", doc.Editor.Caret())
	}
	if err := driver.Key(ui.KeyLeft, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyLeft, ui.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 1, End: 2}) {
		t.Fatalf("keyboard cannot select read-only text: %+v caret=%d focus=%v", doc.Editor.Selection(), doc.Editor.Caret(), driver.Snapshot().Focused)
	}
	state.command("Copy")
	if driver.Clipboard() != "b" || doc.Editor.String() != "abc" {
		t.Fatalf("read-only copy = %q, text=%q", driver.Clipboard(), doc.Editor.String())
	}
}

func TestReadOnlyOpenTabBlocksDirectoryReplacementBeforeDiskWrite(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sample.txt")
	if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.command("Read-Only on Current Document")
	plan, err := searchfiles.PreviewReplace(context.Background(), root, "cat", "dog", document.SearchOptions{}, searchfiles.Options{Pattern: "*.txt"})
	if err != nil || len(plan.Changes) != 1 {
		t.Fatalf("preview = %+v, %v", plan, err)
	}
	state.replacePlan = plan
	state.startReplaceApply()
	if state.filesApplying || !strings.Contains(state.notice, "Read-only") {
		t.Fatalf("read-only directory replacement started: applying=%v notice=%q", state.filesApplying, state.notice)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "cat" {
		t.Fatalf("read-only file changed: %q, %v", data, err)
	}
}

func TestReadOnlyPointerDragAndWordNavigation(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("alpha beta")
	state.command("Read-Only on Current Document")
	doc.Editor.MoveTo(0, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	start, ok := state.codeEditor().CaretBounds()
	if !ok {
		t.Fatal("read-only caret was not painted")
	}
	doc.Editor.MoveTo(5, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	end, ok := state.codeEditor().CaretBounds()
	if !ok {
		t.Fatal("end caret was not painted")
	}
	if err := driver.Press(start.X+0.1, start.Y+start.Height/2, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(end.X+0.1, end.Y+end.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(end.X+0.1, end.Y+end.Height/2, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 5}) {
		t.Fatalf("read-only drag selection = %+v", doc.Editor.Selection())
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	modifier := ui.ModifierControl
	if runtime.GOOS == "darwin" {
		modifier = ui.ModifierAlt
	}
	if err := driver.Key(ui.KeyLeft, modifier); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Caret() != 6 {
		t.Fatalf("read-only word-left caret = %d", doc.Editor.Caret())
	}
	if err := driver.Key(ui.KeyLeft, modifier|ui.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 6}) || doc.Editor.String() != "alpha beta" {
		t.Fatalf("read-only word selection = %+v", doc.Editor.Selection())
	}
}

func TestReadOnlyStatusIsVisibleInCompactWindow(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.command("Read-Only on Current Document")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, label := range driver.Snapshot().Labels {
		if label.Value == "READ ONLY" && label.Bounds.Y >= 400-statusHeight {
			return
		}
	}
	t.Fatal("read-only mode is not visible in the status bar")
}

func TestReadOnlyRejectsStaleNativeInputCallback(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("safe")
	input := state.macroTextInput(doc)
	state.command("Read-Only on Current Document")
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, Text: "bad"}); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "safe" || !state.isReadOnly(doc) {
		t.Fatalf("stale input modified read-only document: %q", doc.Editor.String())
	}
}

func TestReadOnlyAllowsSavingPreexistingDirtyBuffer(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("unsaved")
	state.command("Read-Only on Current Document")
	path := filepath.Join(t.TempDir(), "saved.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Save")
	if data, err := os.ReadFile(path); err != nil || string(data) != "unsaved" || doc.Dirty() || !state.isReadOnly(doc) {
		t.Fatalf("save of read-only buffer = %q, %v, dirty=%v readonly=%v", data, err, doc.Dirty(), state.isReadOnly(doc))
	}
}

func TestReadOnlyBlocksDestructiveBookmarkedLineCommands(t *testing.T) {
	for _, command := range []string{
		"Cut Bookmarked Lines", "Paste to (Replace) Bookmarked Lines",
		"Remove Bookmarked Lines", "Remove Non-Bookmarked Lines",
	} {
		t.Run(command, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("a\nb")
			doc.GoToLine(1)
			state.command("Toggle Bookmark")
			state.command("Read-Only on Current Document")
			driver.SetClipboard("X")
			state.command(command)
			if doc.Editor.String() != "a\nb" || driver.Clipboard() != "X" || len(state.bookmarkLines(doc)) != 1 ||
				!strings.Contains(state.notice, "Read-only") {
				t.Fatalf("command changed read-only bookmarked text: %q, marks=%v, clipboard=%q, notice=%q", doc.Editor.String(), state.bookmarkLines(doc), driver.Clipboard(), state.notice)
			}
		})
	}
}
