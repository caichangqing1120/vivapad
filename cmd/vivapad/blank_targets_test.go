package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestBlankTrimJoinPreserveUntouchedStylesAndFindMark(t *testing.T) {
	for _, scenario := range []struct {
		command, before, after string
	}{
		{"Trim Trailing Space", "  keep \t\n\tkeep  \n", "  keep\n\tkeep\n"},
		{"Trim Leading Space", "  keep \t\n\tkeep  \n", "keep \t\nkeep  \n"},
		{"Trim Leading and Trailing Space", "  keep \t\n\tkeep  \n", "keep\nkeep\n"},
		{"EOL to Space", "keep \n keep\n", "keep  keep "},
		{"Trim both and EOL to Space", "  keep \t\n\tkeep  \n", "keep keep "},
	} {
		t.Run(scenario.command, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert(scenario.before)
			doc.Editor.ClearHistory()
			first := strings.Index(scenario.before, "keep")
			for style := range 5 {
				doc.Editor.SetSelection(text.Range{Start: first, End: first + 4})
				state.styleToken(style, true)
			}
			state.query.Insert("keep")
			state.command("Mark All")
			beforeSpans := append([]text.Span(nil), state.styleSpans(doc, 0)...)
			doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 0})
			state.command(scenario.command)
			first = strings.Index(scenario.after, "keep")
			last := strings.LastIndex(scenario.after, "keep")
			want := []text.Span{{Start: first, End: first + 4}, {Start: last, End: last + 4}}
			if scenario.command == "Trim both and EOL to Space" {
				want = []text.Span{{Start: first, End: last + 4}}
				beforeSpans = []text.Span{{Start: 2, End: 14}}
			}
			state.syncFindMarks(doc)
			if doc.Editor.String() != scenario.after || !slices.Equal(state.markedSpans, want) {
				t.Fatalf("blank command changed untouched Find Mark: %q, %+v", doc.Editor.String(), state.markedSpans)
			}
			for style := range 5 {
				if !slices.Equal(state.styleSpans(doc, style), want) {
					t.Fatalf("blank command lost untouched style %d", style)
				}
			}
			selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
			clickID(t, driver, undoID)
			state.syncFindMarks(doc)
			if doc.Editor.String() != scenario.before || doc.Editor.CanUndo() || !slices.Equal(state.styleSpans(doc, 0), beforeSpans) || !slices.Equal(state.markedSpans, beforeSpans) {
				t.Fatal("blank Undo lost untouched style or Find Mark")
			}
			clickID(t, driver, redoID)
			if doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || !slices.Equal(state.styleSpans(doc, 4), want) {
				t.Fatal("blank Redo lost final selection or indicators")
			}
		})
	}
}

func TestBlankTabReplacesChangedLineStylesButNotUnchangedLines(t *testing.T) {
	for _, scenario := range []struct {
		command, before, after string
	}{
		{"TAB to Space", "a\tkeep\nkeep\n\tkeep", "a   keep\nkeep\n    keep"},
		{"Space to TAB (All)", "a   keep\nkeep\n    keep", "a\tkeep\nkeep\n\tkeep"},
		{"Space to TAB (Leading)", "    keep\nkeep\n    keep", "\tkeep\nkeep\n\tkeep"},
	} {
		t.Run(scenario.command, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert(scenario.before)
			doc.Editor.ClearHistory()
			first := strings.Index(scenario.before, "keep")
			for style := range 5 {
				doc.Editor.SetSelection(text.Range{Start: first, End: first + 4})
				state.styleToken(style, true)
			}
			state.query.Insert("keep")
			state.command("Mark All")
			doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 0})
			state.command(scenario.command)
			middle := strings.Index(scenario.after, "\nkeep") + 1
			want := []text.Span{{Start: middle, End: middle + 4}}
			state.syncFindMarks(doc)
			if doc.Editor.String() != scenario.after || !slices.Equal(state.markedSpans, want) {
				t.Fatalf("TAB source-line replacement = %q marks=%+v, want %+v", doc.Editor.String(), state.markedSpans, want)
			}
			for style := range 5 {
				if !slices.Equal(state.styleSpans(doc, style), want) {
					t.Fatalf("TAB conversion failed source replacement policy for style %d", style)
				}
			}
			clickID(t, driver, undoID)
			middle = strings.Index(scenario.before, "\nkeep") + 1
			beforeWant := []text.Span{{Start: middle, End: middle + 4}}
			state.syncFindMarks(doc)
			if doc.Editor.String() != scenario.before || doc.Editor.CanUndo() || !slices.Equal(state.styleSpans(doc, 0), beforeWant) || !slices.Equal(state.markedSpans, beforeWant) {
				t.Fatal("TAB Undo restored deleted style snapshots or changed the untouched line")
			}
		})
	}
}

func TestBlankTargetsCloneBookmarksAndReadOnly(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert(" left keep \n right keep \n")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 6, End: 10})
	state.styleToken(0, true)
	doc.GoToLine(2)
	state.toggleBookmark()
	state.command("Clone to Other View")
	doc.Editor.MoveTo(0, false)
	state.command("Trim Leading and Trailing Space")
	want := []text.Span{{Start: 5, End: 9}, {Start: 16, End: 20}}
	if state.activePane != 1 || state.paneDocs[0] != doc || state.paneDocs[1] != doc || !slices.Equal(state.bookmarkLines(doc), []int{1}) || !slices.Equal(state.styleSpans(doc, 0), want) || !slices.Equal(state.styleSpans(doc, 0), want) {
		t.Fatal("trim lost shared-clone styles, active ownership or bookmark identity")
	}
	clickID(t, driver, undoID)
	state.command("Read-Only on Current Document")
	cursor := doc.Editor.TrackEdits()
	state.command("Trim both and EOL to Space")
	if doc.Editor.String() != " left keep \n right keep \n" || state.notice != "Read-only document" || !slices.Equal(state.bookmarkLines(doc), []int{1}) {
		t.Fatal("read-only blank command changed the shared source")
	}
	if edits, _, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
		t.Fatal("read-only blank command recorded an edit")
	}
}

func TestBlankCommandsRefuseActiveRectangularSelection(t *testing.T) {
	for _, command := range blankCommands {
		state, driver := startDriver(t)
		doc := state.space.Current()
		doc.Editor.Insert(" \tkeep \n    keep \n")
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(text.Range{Start: 6, End: 2})
		state.rectangles[0] = rectangleSelection{doc: doc, anchor: 2, caret: 12, value: doc.Editor.String(), active: true}
		before, selection, rectangle := doc.Editor.String(), doc.Editor.Selection(), state.rectangles[0]
		cursor := doc.Editor.TrackEdits()
		state.command(command)
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		if doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.Caret() != 2 || doc.Editor.CanUndo() || state.rectangles[0] != rectangle {
			t.Fatalf("%s edited a source rectangle", command)
		}
		if edits, _, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
			t.Fatal("refused rectangle recorded an edit")
		}
	}
}

func TestCaptureBlankTargetsWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_BLANK_TARGET_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_BLANK_TARGET_SHOTS to capture source blank operations")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Name = "blank-targets.txt"
	doc.Editor.Insert("  alpha keep  \n\tleft keep \t\nplain keep\n")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 8, End: 12})
	state.styleToken(0, true)
	doc.Editor.SetSelection(text.Range{Start: 2, End: 7})
	state.styleToken(1, false)
	state.query.Insert("left")
	state.command("Mark All")
	doc.GoToLine(2)
	state.toggleBookmark()
	doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 0})
	state.command("Trim Leading and Trailing Space")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	capture := func(name string, width, height int) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s blank target layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "blank-targets-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	capture("trim-desktop", 1280, 800)
	capture("trim-compact", 640, 400)
	state.command("Undo")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	capture("undo-compact", 640, 400)
	state.command("TAB to Space")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	capture("tab-lines-compact", 640, 400)
	state.command("Clone to Other View")
	capture("clone-compact", 640, 400)
}
