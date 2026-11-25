package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
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

func TestDocumentSummaryCountsMixedLineEndingsAndUnicodeWords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("é_２\r\n猫🙂\nZ\rx\u0301y"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 2, End: 7})
	summary := summaryFor(doc)
	if summary.characters != 9 || summary.words != 4 || summary.lines != 4 || summary.bytes != 22 || summary.selectedCharacters != 5 || summary.selectedBytes != 13 {
		t.Fatalf("mixed summary = %+v", summary)
	}
}

func TestSummaryCountsActiveSelectionAreas(t *testing.T) {
	for _, tc := range []struct {
		name, source        string
		selections          []text.Range
		rectangle           bool
		anchor, caret       int
		chars, bytes, areas int
	}{
		{name: "empty caret", source: "abc"},
		{name: "multiple carets", source: "abc", selections: []text.Range{{Start: 2, End: 2}, {Start: 0, End: 0}}, areas: 2},
		{name: "mixed endings and Unicode", source: "aé\r\n猫b\nz\rc🙂", selections: []text.Range{{Start: 8, End: 10}, {Start: 0, End: 1}, {Start: 6, End: 6}, {Start: 1, End: 3}}, chars: 5, bytes: 10, areas: 4},
		{name: "rectangle with empty row", source: "aé\r\n\n猫b", rectangle: true, anchor: 0, caret: 5, chars: 3, bytes: 6, areas: 1},
		{name: "thin rectangle", source: "abc\ndef", rectangle: true, anchor: 0, caret: 4, areas: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample.txt")
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{path})
			doc := state.space.Current()
			if len(tc.selections) > 0 {
				doc.Editor.SetSelection(tc.selections[len(tc.selections)-1])
				state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), ranges: slices.Clone(tc.selections), active: true}
			}
			if tc.rectangle {
				doc.Editor.MoveTo(tc.caret, false)
				state.rectangles[0] = rectangleSelection{doc: doc, value: doc.Editor.String(), anchor: tc.anchor, caret: tc.caret, active: true}
			}
			selection, rectangle := doc.Editor.Selection(), state.rectangles[0]
			before := doc.Editor.String()
			state.command("Summary...")
			got := state.summary
			if !state.summaryOpen || got.selectedCharacters != tc.chars || got.selectedBytes != tc.bytes || got.ranges != tc.areas {
				t.Fatalf("summary open=%v: %d characters, %d bytes, %d ranges; want %d, %d, %d", state.summaryOpen, got.selectedCharacters, got.selectedBytes, got.ranges, tc.chars, tc.bytes, tc.areas)
			}
			if doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.CanUndo() || doc.Dirty() ||
				!slices.Equal(state.multiSelections[0].ranges, tc.selections) || state.rectangles[0] != rectangle {
				t.Fatal("summary changed editor state")
			}
		})
	}
}

func BenchmarkDocumentSummary(b *testing.B) {
	for _, sample := range []struct{ name, text string }{
		{"ASCII", "alpha beta forty_two 40\n"},
		{"MixedEOL", "alpha βeta ４０\r\n猫🙂 42\nZ\rx\u0301y\n"},
	} {
		b.Run(sample.name, func(b *testing.B) {
			value := strings.Repeat(sample.text, (2<<20)/len(sample.text))
			path := filepath.Join(b.TempDir(), "large.txt")
			if err := os.WriteFile(path, []byte(value), 0600); err != nil {
				b.Fatal(err)
			}
			doc, err := document.Open(path)
			if err != nil {
				b.Fatal(err)
			}
			doc.Editor.SetSelection(text.Range{Start: 10, End: doc.Editor.RuneCount() / 2})
			summaryFor(doc)
			b.SetBytes(int64(len(value)))
			b.ReportAllocs()
			for b.Loop() {
				summaryFor(doc)
			}
		})
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
	clickID(t, driver, menuItemScope.At(viewCommandIndex("Summary...")))
	if !state.summaryOpen || state.summary.characters != 8 || state.summary.selectedCharacters != 2 || state.summary.selectedBytes != 6 {
		t.Fatalf("active-pane summary = %+v, open=%v", state.summary, state.summaryOpen)
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == summaryCloseID && (hit.Bounds.X < 0 || hit.Bounds.Y < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400) {
			t.Fatalf("summary close control clipped: %+v", hit.Bounds)
		}
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.summaryOpen {
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

func TestSummaryDismissalReturnsFocusToActivePane(t *testing.T) {
	for _, method := range []string{"OK", "Escape", "Enter"} {
		t.Run(method, func(t *testing.T) {
			state, driver := startDriver(t)
			left := state.space.Current()
			left.Editor.Insert("left")
			right := state.space.NewDocument()
			right.Editor.Insert("alpha beta")
			right.Editor.ClearHistory()
			state.paneOwner[left], state.paneOwner[right] = 0, 1
			state.paneDocs = [2]*document.Document{left, right}
			state.splitEnabled = true
			state.activatePane(1)
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, rightEditorID)
			selection := text.Range{Start: 0, End: 5}
			right.Editor.SetSelection(selection)
			clickID(t, driver, menuScope.At(3))
			clickID(t, driver, menuItemScope.At(viewCommandIndex("Summary...")))
			if !state.summaryOpen {
				t.Fatal("View menu did not open Summary")
			}
			switch method {
			case "OK":
				clickID(t, driver, summaryCloseID)
			case "Escape":
				if err := driver.Key(gfx.KeyEscape, 0); err != nil {
					t.Fatal(err)
				}
			case "Enter":
				if err := driver.Key(gfx.KeyEnter, 0); err != nil {
					t.Fatal(err)
				}
			}
			if state.summaryOpen || right.Editor.Selection() != selection || right.Editor.CanUndo() {
				t.Fatal("Summary dismissal changed editor state")
			}
			requireEditorCommandFocus(t, driver, rightEditorID)
			if err := driver.Type("q"); err != nil || right.Editor.String() != "q beta" || left.Editor.String() != "left" {
				t.Fatalf("first input after Summary = %q, left %q, error %v", right.Editor.String(), left.Editor.String(), err)
			}
			if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || right.Editor.String() != "alpha beta" || right.Editor.CanUndo() {
				t.Fatalf("Undo after Summary = %q, handled %v, error %v", right.Editor.String(), handled, err)
			}
		})
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
