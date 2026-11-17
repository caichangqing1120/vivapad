package main

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
)

func TestMoveLinesSelectsWholeMovedBlock(t *testing.T) {
	for _, test := range []struct {
		name      string
		selection text.Range
		up        bool
		wantText  string
		wantRange text.Range
	}{
		{"forward partial selection", text.Range{Start: 2, End: 3}, false, "a\nc\nbb\nd", text.Range{Start: 4, End: 7}},
		{"reverse partial selection", text.Range{Start: 3, End: 2}, false, "a\nc\nbb\nd", text.Range{Start: 4, End: 7}},
		{"collapsed caret", text.Range{Start: 3, End: 3}, false, "a\nc\nbb\nd", text.Range{Start: 4, End: 7}},
		{"selection ends at next line start", text.Range{Start: 2, End: 5}, false, "a\nc\nbb\nd", text.Range{Start: 4, End: 7}},
		{"move up", text.Range{Start: 5, End: 6}, true, "a\nc\nbb\nd", text.Range{Start: 2, End: 4}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert("a\nbb\nc\nd")
			doc.Editor.ClearHistory()
			doc.Editor.SetSelection(test.selection)
			command := components.CodeEditorMoveLinesDown
			if test.up {
				command = components.CodeEditorMoveLinesUp
			}
			state.applyCodeCommand(doc, command)
			if got := doc.Editor.String(); got != test.wantText {
				t.Fatalf("moved text = %q", got)
			}
			// Scintilla's MoveSelectedLines ends with a reversed selection of
			// the whole inserted line block, regardless of the input shape.
			if got := doc.Editor.Selection(); got != test.wantRange || doc.Editor.Caret() != test.wantRange.Start {
				t.Fatalf("moved selection = %+v, caret=%d; want %+v with caret at start", got, doc.Editor.Caret(), test.wantRange)
			}
			if !doc.Editor.Undo() || doc.Editor.String() != "a\nbb\nc\nd" {
				t.Fatalf("move undo = %q", doc.Editor.String())
			}
		})
	}
}

func TestMoveLinesSelectsMovedBlockAtEndOfDocument(t *testing.T) {
	for _, test := range []struct {
		name, before, after string
		caret               int
		up                  bool
		selection           text.Range
	}{
		{"down across unterminated EOF", "a\nb", "b\na", 0, false, text.Range{Start: 2, End: 3}},
		{"up from unterminated EOF", "a\nb", "b\na", 2, true, text.Range{Start: 0, End: 2}},
		{"up from empty terminal line", "a\n", "\na", 2, true, text.Range{Start: 0, End: 1}},
		{"Unicode down across EOF", "猫\n犬", "犬\n猫", 0, false, text.Range{Start: 2, End: 3}},
		{"Unicode up from EOF", "猫\n犬", "犬\n猫", 2, true, text.Range{Start: 0, End: 2}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert(test.before)
			doc.Editor.ClearHistory()
			doc.Editor.MoveTo(test.caret, false)
			state.moveLinesWithBookmarks(doc, test.up)
			if got := doc.Editor.String(); got != test.after {
				t.Fatalf("terminal move = %q, want %q", got, test.after)
			}
			if got := doc.Editor.Selection(); got != test.selection || doc.Editor.Caret() != test.selection.Start {
				t.Fatalf("terminal selection = %+v, caret=%d; want %+v with caret at start", got, doc.Editor.Caret(), test.selection)
			}
			if !doc.Editor.Undo() || doc.Editor.String() != test.before {
				t.Fatalf("terminal move undo = %q", doc.Editor.String())
			}
		})
	}
}
