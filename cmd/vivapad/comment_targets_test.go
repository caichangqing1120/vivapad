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

func TestCommentCommandsPreserveUntouchedStylesAndFindMark(t *testing.T) {
	for _, scenario := range []struct {
		command, language, before, after string
	}{
		{"Single Line Comment", ".go", "α keep β\nnext keep end", "// α keep β\n// next keep end"},
		{"Toggle Single Line Comment", ".go", "// α keep β\nnext keep end", "α keep β\n// next keep end"},
		{"Single Line Uncomment", ".go", "// α keep β\n// next keep end", "α keep β\nnext keep end"},
		{"Single Line Comment", ".css", "α keep β\nnext keep end", "/* α keep β */\n/* next keep end */"},
		{"Toggle Single Line Comment", ".css", "/* α keep β */\nnext keep end", "α keep β\n/* next keep end */"},
		{"Block Comment", ".go", "α keep β\nnext keep end", "/* α keep β\nnext keep end */"},
		{"Block Uncomment", ".go", "/* α keep β */ gap /* next keep end */", "α keep β gap next keep end"},
	} {
		t.Run(scenario.language+"/"+scenario.command, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert(scenario.before)
			doc.Editor.ClearHistory()
			state.languages[doc] = scenario.language
			state.configureCode(doc)
			first := strings.Index(scenario.before, "keep")
			doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneIndex(first), End: doc.Editor.RuneIndex(first + 4)})
			state.styleToken(0, true)
			state.query.Insert("keep")
			state.command("Mark All")
			beforeStyles := append([]text.Span(nil), state.styleSpans(doc, 0)...)
			if len(beforeStyles) != 2 || len(state.markedSpans) != 2 {
				t.Fatalf("comment fixture did not mark both tokens: styles=%+v find=%+v", beforeStyles, state.markedSpans)
			}
			doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 0})
			state.command(scenario.command)
			first = strings.Index(scenario.after, "keep")
			last := strings.LastIndex(scenario.after, "keep")
			want := []text.Span{{Start: first, End: first + 4}, {Start: last, End: last + 4}}
			state.syncFindMarks(doc)
			if doc.Editor.String() != scenario.after || !slices.Equal(state.styleSpans(doc, 0), want) || !slices.Equal(state.markedSpans, want) {
				t.Fatalf("comment replaced untouched content: %q styles=%+v find=%+v, want %+v", doc.Editor.String(), state.styleSpans(doc, 0), state.markedSpans, want)
			}
			selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
			clickID(t, driver, undoID)
			state.syncFindMarks(doc)
			if doc.Editor.String() != scenario.before || !slices.Equal(state.styleSpans(doc, 0), beforeStyles) || !slices.Equal(state.markedSpans, beforeStyles) {
				t.Fatal("comment Undo lost untouched styles or Find Mark")
			}
			clickID(t, driver, redoID)
			state.syncFindMarks(doc)
			if doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || !slices.Equal(state.styleSpans(doc, 0), want) || !slices.Equal(state.markedSpans, want) {
				t.Fatal("comment Redo lost mapped selection or untouched indicators")
			}
		})
	}
}

func TestCommentTargetsCloneBookmarksAndReadOnly(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	before := "left keep\nright keep"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	state.languages[doc] = ".go"
	state.configureCode(doc)
	for style := range 5 {
		doc.Editor.SetSelection(text.Range{Start: 5, End: 9})
		state.styleToken(style, true)
	}
	state.query.Insert("keep")
	state.command("Mark All")
	doc.GoToLine(2)
	state.toggleBookmark()
	state.command("Clone to Other View")
	doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 0})
	state.command("Single Line Comment")
	want := []text.Span{{Start: 8, End: 12}, {Start: 22, End: 26}}
	if state.activePane != 1 || state.paneDocs[0] != doc || state.paneDocs[1] != doc || doc.Editor.String() != "// left keep\n// right keep" {
		t.Fatal("comment did not use the active shared clone")
	}
	for style := range 5 {
		if !slices.Equal(state.styleSpans(doc, style), want) || !slices.Equal(state.styleSpans(doc, style), want) {
			t.Fatalf("style %d was lost or applied twice across clone reads", style)
		}
	}
	if !slices.Equal(state.bookmarkLines(doc), []int{1}) {
		t.Fatal("comment changed bookmark line identity")
	}
	clickID(t, driver, undoID)
	if doc.Editor.String() != before || doc.Editor.CanUndo() || !slices.Equal(state.bookmarkLines(doc), []int{1}) {
		t.Fatal("clone Undo did not restore one comment action and its bookmark")
	}
	clickID(t, driver, redoID)
	state.command("Read-Only on Current Document")
	cursor := doc.Editor.TrackEdits()
	state.command("Single Line Uncomment")
	if doc.Editor.String() != "// left keep\n// right keep" || !slices.Equal(state.styleSpans(doc, 4), want) || state.notice != "Read-only document" {
		t.Fatal("read-only comment changed clone text or styles")
	}
	if edits, _, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
		t.Fatal("read-only comment added edit primitives")
	}
}

func TestCaptureCommentTargetsWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_COMMENT_TARGET_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_COMMENT_TARGET_SHOTS to capture granular comments")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Name = "comment-targets.go"
	doc.Editor.Insert("alpha keep beta\nleft keep right")
	doc.Editor.ClearHistory()
	state.languages[doc] = ".go"
	state.configureCode(doc)
	doc.Editor.SetSelection(text.Range{Start: 6, End: 10})
	state.styleToken(0, true)
	doc.Editor.SetSelection(text.Range{Start: 11, End: 15})
	state.styleToken(1, false)
	state.query.Insert("alpha")
	state.command("Mark All")
	doc.GoToLine(2)
	state.toggleBookmark()
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	state.command("Single Line Comment")
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
			t.Fatalf("%s comment target layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "comment-targets-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	capture("line-desktop", 1280, 800)
	capture("line-compact", 640, 400)
	state.command("Undo")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	capture("undo-compact", 640, 400)
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	state.command("Block Comment")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	capture("block-compact", 640, 400)
	state.command("Clone to Other View")
	capture("clone-compact", 640, 400)
}
