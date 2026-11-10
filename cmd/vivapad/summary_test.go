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
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func TestDocumentSummaryCountsUnicodeSelectionAndLogicalLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("aé\r\nb_c 42\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	summary := summaryFor(doc)
	if summary.path != doc.Path || summary.modified == "" || summary.characters != 8 || summary.words != 3 || summary.lines != 3 || summary.bytes != 13 || summary.selectedCharacters != 2 || summary.selectedBytes != 3 || summary.ranges != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	doc.Editor.SetSelection(text.Range{Start: 1, End: 3})
	if summary = summaryFor(doc); summary.selectedBytes != 4 {
		t.Fatalf("selected CRLF bytes = %d, want 4", summary.selectedBytes)
	}
	untitled := document.New("new 1")
	untitled.Editor.Insert("Hello, 世界\n")
	summary = summaryFor(untitled)
	if summary.path != "" || summary.created != "" || summary.modified != "" || summary.characters != 9 || summary.words != 2 || summary.lines != 2 || summary.bytes != 14 || summary.selectedCharacters != 0 || summary.selectedBytes != 0 {
		t.Fatalf("untitled summary = %+v", summary)
	}
}

func TestSummaryModalUsesActiveSplitViewAndPreservesEditingState(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Name = "left.txt"
	left.Editor.Insert("left\n")
	right := state.space.NewDocument()
	right.Name = "right.txt"
	right.Editor.Insert("right 世界\n")
	right.Editor.SetSelection(text.Range{Start: 6, End: 8})
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 1
	state.space.Select(state.documentIndex(right))
	before := right.Editor.Selection()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(22))
	if !state.summaryOpen || state.summary.characters != 8 || state.summary.selectedCharacters != 2 || state.summary.selectedBytes != 6 {
		t.Fatalf("active-pane summary = %+v, open=%v", state.summary, state.summaryOpen)
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == summaryCloseID && (hit.Bounds.X < 0 || hit.Bounds.Y < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400) {
			t.Fatalf("summary close control clipped: %+v", hit.Bounds)
		}
	}
	if err := driver.Key(ui.KeyEscape, 0); err != nil || state.summaryOpen {
		t.Fatalf("Escape did not dismiss summary: %v", err)
	}
	state.command("Summary...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, summaryCloseID)
	if state.summaryOpen || right.Editor.String() != "right 世界\n" || right.Editor.Selection() != before || !right.Dirty() || left.Editor.String() != "left\n" || !strings.Contains(state.space.Current().Name, "right") {
		t.Fatal("summary dismissal changed the active document")
	}
}

func TestSummarySnapshotMissingFileAndModalInputSafety(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver.Driver(), editorID)
	doc := state.space.Current()
	state.command("Summary...")
	if state.summary.characters != 8 {
		t.Fatal("summary did not snapshot the initial text")
	}
	doc.Editor.Insert(" more")
	if state.summary.characters != 8 {
		t.Fatal("summary changed while its modal remained open")
	}
	state.invalidator.Invalidate()
	if err := driver.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	before := doc.Editor.String()
	if err := driver.Type("discarded"); !errors.Is(err, app.ErrNoTextInput) {
		t.Fatalf("modal text input = %v, want no focused field", err)
	}
	if doc.Editor.String() != before {
		t.Fatalf("modal allowed background typing: %q -> %q", before, doc.Editor.String())
	}
	state.summaryOpen = false
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	missing := summaryFor(doc)
	if missing.path != doc.Path || missing.created != "" || missing.modified != "" {
		t.Fatalf("missing disk file invented metadata: %+v", missing)
	}
}

func TestCaptureSummaryWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_SUMMARY_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_SUMMARY_SHOTS to capture Summary dialog")
	}
	path := filepath.Join(t.TempDir(), "a-very-long-saved-document-name-for-summary.txt")
	if err := os.WriteFile(path, []byte("alpha 世界\r\nbeta_2\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.space.Current().Editor.SetSelection(text.Range{Start: 0, End: 8})
	state.command("Summary...")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 800},
		{"compact", 640, 400},
	} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "summary-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}
