package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vivaui/viva/headless"
	"vivapad/internal/searchfiles"
)

func TestBookmarkReloadRebindsSharedCloneWithoutChangingText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.txt")
	if err := os.WriteFile(path, []byte("first\nmarked\nlast"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	old := state.space.Current()
	state.command("Clone to Other View")
	old.GoToLine(2)
	state.command("Toggle Bookmark")
	current := state.reloadDocument(state.space.Active, false)
	if current == nil || current == old || state.paneDocument(0) != current || state.paneDocument(1) != current ||
		!reflect.DeepEqual(state.bookmarkLines(current), []int{1}) || state.bookmarks[old] != nil {
		t.Fatalf("clone reload markers: current=%v panes=%v marks=%v", current, state.paneDocs, state.bookmarkLines(current))
	}
	current.GoToLine(1)
	state.command("Next Bookmark")
	if bookmarkLine(current) != 1 || current.Dirty() {
		t.Fatalf("navigation after reload: caret=%d dirty=%v", bookmarkLine(current), current.Dirty())
	}
}

func TestBookmarkReloadMapsUniqueSurvivingLinesAndDropsDeleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changed.txt")
	if err := os.WriteFile(path, []byte("first\nkeep\nremoved\nlast"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	old := state.space.Current()
	for _, line := range []int{2, 3} {
		old.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	if err := os.WriteFile(path, []byte("new\nfirst\nkeep\nlast"), 0600); err != nil {
		t.Fatal(err)
	}
	current := state.reloadDocument(state.space.Active, false)
	if current == nil || !reflect.DeepEqual(state.bookmarkLines(current), []int{2}) || state.bookmarks[old] != nil {
		t.Fatalf("changed reload markers: current=%v marks=%v", current, state.bookmarkLines(current))
	}
	if !reflect.DeepEqual(current.Editor.String(), "new\nfirst\nkeep\nlast") || current.Dirty() || current.Editor.CanUndo() {
		t.Fatalf("changed reload editor: %q dirty=%v undo=%v", current.Editor.String(), current.Dirty(), current.Editor.CanUndo())
	}
}

func TestBookmarkReloadDoesNotTransferAmbiguousDuplicateOrFailedOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ambiguous.txt")
	if err := os.WriteFile(path, []byte("same\nsame"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	old := state.space.Current()
	old.GoToLine(2)
	state.command("Toggle Bookmark")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if state.reloadDocument(state.space.Active, false) != nil || state.space.Current() != old || !reflect.DeepEqual(state.bookmarkLines(old), []int{1}) {
		t.Fatal("failed reload changed the document or its bookmark")
	}
	if err := os.WriteFile(path, []byte("same\nsame\nsame"), 0600); err != nil {
		t.Fatal(err)
	}
	current := state.reloadDocument(state.space.Active, false)
	if current == nil || len(state.bookmarkLines(current)) != 0 {
		t.Fatalf("ambiguous duplicate inherited a different line: %v", state.bookmarkLines(current))
	}
}

func TestBookmarkSearchReplaceReloadRetainsUnaffectedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replace.txt")
	if err := os.WriteFile(path, []byte("header\nkeep\nold"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	old := state.space.Current()
	old.GoToLine(2)
	state.command("Toggle Bookmark")
	if err := os.WriteFile(path, []byte("header\nkeep\nnew"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.refreshReplacedTabs(searchfiles.ApplyResult{Paths: []string{old.Path}}); err != nil {
		t.Fatal(err)
	}
	current := state.space.Current()
	if current == old || !reflect.DeepEqual(state.bookmarkLines(current), []int{1}) || state.bookmarks[old] != nil {
		t.Fatalf("search replace reload markers: current=%v marks=%v", current, state.bookmarkLines(current))
	}
}

func TestBookmarkReloadKeepsStableDuplicateBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "duplicates.txt")
	if err := os.WriteFile(path, []byte("same\nsame\nold"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	if err := os.WriteFile(path, []byte("same\nsame\nnew"), 0600); err != nil {
		t.Fatal(err)
	}
	current := state.reloadDocument(state.space.Active, false)
	if current == nil || !reflect.DeepEqual(state.bookmarkLines(current), []int{1}) {
		t.Fatalf("stable duplicate block lost marker: %v", state.bookmarkLines(current))
	}
}

func TestCaptureBookmarkedReloadWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_BOOKMARK_RELOAD_SHOT")
	if output == "" {
		t.Skip("set VIVAPAD_BOOKMARK_RELOAD_SHOT to capture reloaded bookmarks")
	}
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("start\nkeep\nend"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.command("Clone to Other View")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	if err := os.WriteFile(path, []byte("inserted\nstart\nkeep\nend"), 0600); err != nil {
		t.Fatal(err)
	}
	if state.reloadDocument(state.space.Active, false) == nil {
		t.Fatal(state.notice)
	}
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("reloaded bookmark layout: %+v", problems)
	}
	if err := session.Screenshot(output); err != nil {
		t.Fatal(err)
	}
}
