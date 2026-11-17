package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestRectangleSortRoutesMenuToSourceColumnKeys(t *testing.T) {
	for _, command := range []string{"Sort Lines Ascending", "Sort Ascending (Ignore Case)", "Sort As Integers Ascending", "Sort As Decimals (Dot) Ascending", "Sort As Decimals (Comma) Ascending", "Sort By Length Ascending", "Reverse Line Order"} {
		t.Run(command, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			before := "outside keep\nz 22 keep\na 1 keep\ntail keep"
			want := "outside keep\na 1 keep\nz 22 keep\ntail keep"
			doc.Editor.Insert(before)
			doc.Editor.ClearHistory()
			for style := range 5 {
				doc.Editor.SetSelection(text.Range{Start: 8, End: 12})
				state.styleToken(style, true)
			}
			state.query.Insert("keep")
			state.command("Mark All")
			anchor, caret := len("outside keep\nz "), len("outside keep\nz 22 keep\na ")
			state.rectangles[0] = rectangleSelection{doc: doc, anchor: anchor, caret: caret, value: before, active: true}
			doc.Editor.MoveTo(caret, false)
			state.command(command)
			if doc.Editor.String() != want {
				t.Fatalf("rectangle command sorted outside its column-key block: %q", doc.Editor.String())
			}
			selection := state.rectangles[0]
			if !selection.active || selection.anchor != len("outside keep\n") || selection.caret != selection.anchor || selection.value != want {
				t.Fatalf("rectangle did not follow original whole-target movement: %+v", selection)
			}
			state.syncFindMarks(doc)
			spans := []text.Span{{Start: 8, End: 12}, {Start: strings.LastIndex(want, "keep"), End: len(want)}}
			for style := range 5 {
				if !slices.Equal(state.styleSpans(doc, style), spans) {
					t.Fatalf("sort restored style %d inside replaced block: %+v", style, state.styleSpans(doc, style))
				}
			}
			if !slices.Equal(state.markedSpans, spans) {
				t.Fatalf("sort restored Find Mark inside replaced block: %+v", state.markedSpans)
			}
			clickID(t, driver, undoID)
			if doc.Editor.String() != before || doc.Editor.CanUndo() || !state.rectangles[0].active || state.rectangles[0].anchor != anchor || state.rectangles[0].caret != caret {
				t.Fatal("Undo lost the owning rectangle")
			}
			clickID(t, driver, redoID)
			if doc.Editor.String() != want || snapshotRectangle(state.rectangles[0]) != snapshotRectangle(selection) {
				t.Fatal("Redo lost the resulting rectangle")
			}
		})
	}
}

func TestRectangleSortNoOpFailureAndReadOnlyKeepOwningState(t *testing.T) {
	for _, kind := range []string{"no-op", "numeric error", "read-only", "composition", "restriction"} {
		t.Run(kind, func(t *testing.T) {
			state, _ := startDriver(t)
			doc := state.space.Current()
			before := "z 2\na 1"
			if kind == "no-op" {
				before = "z 1\na 2"
			} else if kind == "numeric error" {
				before = "z --\na 1"
			}
			doc.Editor.Insert(before)
			doc.Editor.ClearHistory()
			caret := len(before) - 1
			doc.Editor.MoveTo(caret, false)
			state.rectangles[0] = rectangleSelection{doc: doc, anchor: 2, caret: caret, value: before, active: true}
			state.readOnly[doc] = kind == "read-only"
			if kind == "composition" {
				state.rectangles[0].composing = true
			}
			if kind == "restriction" {
				doc.Editor.Restrict(func(rune) bool { return true }, 0)
			}
			previous := snapshotRectangle(state.rectangles[0])
			cursor := doc.Editor.TrackEdits()
			command := "Sort Lines Ascending"
			if kind == "numeric error" {
				command = "Sort As Decimals (Dot) Ascending"
			}
			state.command(command)
			edits, _, complete := doc.Editor.EditsSince(cursor, nil)
			if doc.Editor.String() != before || doc.Editor.Caret() != caret || doc.Editor.CanUndo() || snapshotRectangle(state.rectangles[0]) != previous || len(state.rectangleHistory[0]) != 0 || len(edits) != 0 || !complete {
				t.Fatalf("%s changed a refused/no-op rectangle", kind)
			}
		})
	}
}

func TestRectangleSortCloneUndoKeepsOwnerAndMenuTarget(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	before := "outside\nz 2\na 1\ntail"
	want := "outside\na 1\nz 2\ntail"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	state.command("Clone to Other View")
	state.activatePane(0)
	anchor, caret := len("outside\nz "), len("outside\nz 2\na ")
	doc.Editor.MoveTo(caret, false)
	state.rectangles[0] = rectangleSelection{doc: doc, anchor: anchor, caret: caret, value: before, active: true}
	state.command("Line Operations")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, lineScope.At(0))
	if doc.Editor.String() != want || state.rectangles[1].active || !state.rectangles[0].active {
		t.Fatalf("clone menu sort = %q", doc.Editor.String())
	}
	state.activatePane(1)
	clickID(t, driver, rightEditorID)
	if _, err := driver.ShortcutChord(primary(), 'z'); err != nil || doc.Editor.String() != before || state.activePane != 1 || !state.rectangles[0].active || state.rectangles[1].active || state.rectangles[0].anchor != anchor || state.rectangles[0].caret != caret {
		t.Fatalf("clone Undo lost rectangle ownership: %q %v", doc.Editor.String(), err)
	}
	if _, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'z'); err != nil || doc.Editor.String() != want || state.rectangles[0].anchor != len("outside\n") || state.rectangles[1].active {
		t.Fatalf("clone Redo lost rectangle ownership: %q %v", doc.Editor.String(), err)
	}
}

func TestRectangleSortAgainAfterCollapseSortsWholeDocument(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	before := "outside\nz 2\na 1\ntail"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	caret := len("outside\nz 2\na ")
	doc.Editor.MoveTo(caret, false)
	state.rectangles[0] = rectangleSelection{doc: doc, anchor: len("outside\nz "), caret: caret, value: before, active: true}
	state.command("Sort Lines Ascending")
	state.command("Sort Lines Ascending")
	if doc.Editor.String() != "a 1\noutside\ntail\nz 2" {
		t.Fatalf("collapsed source sort kept a stale rectangle scope: %q", doc.Editor.String())
	}
}
