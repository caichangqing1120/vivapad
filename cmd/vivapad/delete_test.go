package main

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/macro"
)

func TestEditDeleteRemovesNextGraphemeOrSelectionWithoutClipboard(t *testing.T) {
	family := "👩‍👩‍👧‍👦"
	for _, selection := range []bool{false, true} {
		state, driver := startDriver(t)
		doc := state.space.Current()
		original := "A" + family + "B"
		doc.Editor.Insert(original)
		doc.Editor.ClearHistory()
		if selection {
			doc.Editor.SetSelection(text.Range{Start: 1, End: 1 + len([]rune(family))})
		} else {
			doc.Editor.MoveTo(1, false)
		}
		driver.SetClipboard("prior")
		state.command("Delete")
		if doc.Editor.String() != "AB" || driver.Clipboard() != "prior" {
			t.Fatalf("Delete selection=%v = %q, clipboard=%q", selection, doc.Editor.String(), driver.Clipboard())
		}
		if !doc.Editor.Undo() || doc.Editor.String() != original || doc.Editor.CanUndo() {
			t.Fatalf("Delete selection=%v was not one undo unit: %q", selection, doc.Editor.String())
		}
	}
}

func TestEditDeleteAtEOFIsNoOpAndPreservesLineEnding(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb")
	doc.SetLineEnding(document.CRLF)
	doc.Editor.ClearHistory()
	state.command("Delete")
	if doc.Editor.String() != "a\nb" || doc.Editor.CanUndo() || doc.LineEnding() != document.CRLF {
		t.Fatalf("EOF Delete changed document: %q", doc.Editor.String())
	}
	doc.Editor.MoveTo(1, false)
	state.command("Delete")
	if doc.Editor.String() != "ab" || doc.LineEnding() != document.CRLF {
		t.Fatalf("newline Delete = %q, format=%v", doc.Editor.String(), doc.LineEnding())
	}
}

func TestEditDeleteMergesBookmarksFromEitherSideOfDeletedLF(t *testing.T) {
	for _, markedLine := range []int{1, 2} {
		state := newModel()
		doc := state.space.Current()
		doc.Editor.Insert("same\nsame")
		doc.GoToLine(markedLine)
		state.command("Toggle Bookmark")
		doc.Editor.SetSelection(text.Range{Start: 0, End: 5})
		state.command("Delete")
		marks := state.bookmarkLines(doc)
		if doc.Editor.String() != "same" || !reflect.DeepEqual(marks, []int{0}) {
			t.Fatalf("Delete line %d transferred bookmark: %q, %v", markedLine, doc.Editor.String(), marks)
		}
	}
}

func TestEditDeleteMenuAndMacroRecordReplay(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("abc")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	state.command("Record Macro")
	clickID(t, driver, menuScope.At(1))
	bounds := revealEditMenuItem(t, driver, 14)
	if bounds.Y+bounds.Height > 400-statusHeight {
		t.Fatalf("Delete menu row clipped: %+v", bounds)
	}
	clickID(t, driver, menuItemScope.At(14))
	state.command("Stop Recording")
	if doc.Editor.String() != "ac" || len(state.macroLast) != 1 || state.macroLast[0] != (macro.Step{Kind: macro.TextStep, TextKind: app.TextCommandEvent, Command: app.TextDeleteForward}) {
		t.Fatalf("menu Delete macro = %q, steps=%+v, notice=%q", doc.Editor.String(), state.macroLast, state.notice)
	}
	if !doc.Editor.Undo() {
		t.Fatal("Delete could not undo")
	}
	doc.Editor.MoveTo(1, false)
	state.command("Playback Macro")
	if doc.Editor.String() != "ac" {
		t.Fatalf("Delete macro replay = %q", doc.Editor.String())
	}
}

func TestEditDeleteNoOpDoesNotRecordMacroStep(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("end")
	state.command("Record Macro")
	state.command("Delete")
	state.command("Stop Recording")
	if state.macroRecording || len(state.macroLast) != 0 || doc.Editor.String() != "end" {
		t.Fatalf("EOF Delete recorded a macro edit: %+v, text=%q", state.macroLast, doc.Editor.String())
	}
}
