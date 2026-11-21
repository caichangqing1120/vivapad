package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/document"
)

func noticeDocument(t *testing.T) (*model, *app.Driver, *document.Document) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notice.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	return state, driver, doc
}

func TestNoChangeNoticeExpiresAfterEditingItsSource(t *testing.T) {
	state, driver, doc := noticeDocument(t)
	clickID(t, driver, editorID)
	state.command("Next Change")
	if state.notice != "No changes to navigate" {
		t.Fatalf("initial notice = %q", state.notice)
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(2, false)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.notice != "No changes to navigate" {
		t.Fatalf("caret-only move invalidated the no-change notice: %q", state.notice)
	}
	doc.Editor.MoveTo(0, false)
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != "Xone\ntwo\n" {
		t.Fatalf("real editor input = %q", got)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.notice == "No changes to navigate" {
		t.Fatal("a new changed line still reports no changes")
	}
	if state.noChangeNoticeDoc != nil {
		t.Fatal("an expired notice still retains its source document")
	}
}

func TestNoChangeNoticeBelongsToTheActiveDocument(t *testing.T) {
	state, driver, right := noticeDocument(t)
	leftPath := filepath.Join(t.TempDir(), "left.txt")
	if err := os.WriteFile(leftPath, []byte("left\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(leftPath); err != nil {
		t.Fatal(err)
	}
	left := state.space.Current()
	state.showInActivePane(left)
	state.command("Split View")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	state.command("Next Change")
	if state.notice != "No changes to navigate" {
		t.Fatalf("left no-change notice = %q", state.notice)
	}
	right.Editor.Insert("R")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.notice != "No changes to navigate" {
		t.Fatalf("other document's edit cleared the active notice: %q", state.notice)
	}
	clickID(t, driver, editorID)
	if state.space.Current() != right {
		t.Fatal("right pane did not become active")
	}
	if state.notice == "No changes to navigate" {
		t.Fatal("left document's no-change notice leaked into right pane")
	}
	if state.noChangeNoticeDoc != nil {
		t.Fatal("switching panes left the old document in the expired notice")
	}
	state.notice = "Settings not saved: storage unavailable"
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.notice != "Settings not saved: storage unavailable" {
		t.Fatalf("unrelated error notice was dropped: %q", state.notice)
	}
}

func TestNoChangeNoticeReleasesClosedSource(t *testing.T) {
	for _, notice := range []string{"No changes to navigate", "Settings not saved: storage unavailable"} {
		t.Run(notice, func(t *testing.T) {
			state, _, doc := noticeDocument(t)
			state.command("Next Change")
			if state.noChangeNoticeDoc != doc {
				t.Fatal("precondition: the no-change notice has no source")
			}
			state.notice = notice
			state.closeTab(state.documentIndex(doc))
			if state.documentIndex(doc) >= 0 || state.noChangeNoticeDoc != nil {
				t.Fatal("closing the source left it attached to the status bar")
			}
			want := notice
			if notice == "No changes to navigate" {
				want = ""
			}
			if state.notice != want {
				t.Fatalf("notice after closing source = %q, want %q", state.notice, want)
			}
		})
	}
}
