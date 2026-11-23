package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func BenchmarkMultiSelectionReplaceTwoRanges(b *testing.B) {
	suffix := strings.Repeat("z", 512<<10)
	initial := "a a " + suffix
	want := "X X " + suffix
	ranges := []text.Range{{Start: 0, End: 1}, {Start: 2, End: 3}}
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		state := newModel()
		doc := state.space.Current()
		doc.Editor.Insert(initial)
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(ranges[1])
		state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), active: true, ranges: ranges}
		b.StartTimer()
		changed, err := state.editMultiSelection(0, "X", multiInsert)
		b.StopTimer()
		if err != nil || !changed || doc.Editor.String() != want {
			b.Fatalf("multi-selection edit: changed=%v, error=%v", changed, err)
		}
		b.StartTimer()
	}
}

func TestMultiSelectionNativeTypingAndUndoAreAtomic(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat cat")
	clickID(t, driver, editorID)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	doc.Editor.ClearHistory()
	state.multiSelectAll(document.SearchOptions{})
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "X X X" || doc.Editor.CanUndo() == false || len(state.multiSelections[0].ranges) != 3 {
		t.Fatalf("multi-caret typing = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Undo")
	if doc.Editor.String() != "cat cat cat" || doc.Editor.CanUndo() || !reflect.DeepEqual(state.multiSelections[0].ranges, []text.Range{
		{Start: 0, End: 3}, {Start: 4, End: 7}, {Start: 8, End: 11},
	}) {
		t.Fatalf("multi-caret undo = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Redo")
	if doc.Editor.String() != "X X X" || len(state.multiSelections[0].ranges) != 3 {
		t.Fatalf("multi-caret redo = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "  " {
		t.Fatalf("multi-caret backspace = %q", doc.Editor.String())
	}
}

func TestMultiSelectionGraphemeBackspaceAndMixedLineBookmarks(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("e\u0301 z e\u0301")
	doc.Editor.ClearHistory()
	state.multiSelections[0] = multiSelectionState{
		doc: doc, value: doc.Editor.String(), active: true,
		ranges: []text.Range{{Start: 2, End: 2}, {Start: 7, End: 7}},
	}
	if changed, err := state.editMultiSelection(0, "", multiDeleteBackward); err != nil || !changed || doc.Editor.String() != " z " {
		t.Fatalf("grapheme multi-backspace = %q, %v", doc.Editor.String(), err)
	}
	state.command("Undo")
	if doc.Editor.String() != "e\u0301 z e\u0301" || doc.Editor.CanUndo() {
		t.Fatalf("grapheme deletion undo = %q", doc.Editor.String())
	}
	other := state.space.NewDocument()
	other.Editor.Insert("a\nb\nc\n")
	other.GoToLine(2)
	state.command("Toggle Bookmark")
	other.Editor.ClearHistory()
	state.multiSelections[0] = multiSelectionState{
		doc: other, value: other.Editor.String(), active: true,
		ranges: []text.Range{{Start: 0, End: 1}, {Start: 4, End: 5}},
	}
	other.Editor.SetSelection(text.Range{Start: 4, End: 5})
	if changed, err := state.editMultiSelection(0, "X\nY", multiInsert); err != nil || !changed || other.Editor.String() != "X\nY\nb\nX\nY\n" ||
		!reflect.DeepEqual(state.bookmarkLines(other), []int{2}) {
		t.Fatalf("multiline multi-edit = %q, marks=%v, err=%v", other.Editor.String(), state.bookmarkLines(other), err)
	}
	state.command("Undo")
	if other.Editor.String() != "a\nb\nc\n" || !reflect.DeepEqual(state.bookmarkLines(other), []int{1}) {
		t.Fatalf("multiline multi-undo = %q, marks=%v", other.Editor.String(), state.bookmarkLines(other))
	}
}

func TestMultiSelectionIMEFallsBackToSingleCaretWithoutDuplicatingPreedit(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("a a")
	clickID(t, driver, editorID)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	state.multiSelectAll(document.SearchOptions{})
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Compose("猫"); err != nil {
		t.Fatal(err)
	}
	if state.multiSelections[0].active || doc.Editor.String() == "猫 猫" {
		t.Fatalf("IME preedit duplicated across ranges: %q, state=%+v", doc.Editor.String(), state.multiSelections[0])
	}
	if err := driver.Compose(""); err != nil {
		t.Fatal(err)
	}
}

func TestMultiSelectionCopyCutAndRedactUseEveryRange(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("red red red")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	doc.Editor.ClearHistory()
	state.multiSelectAll(document.SearchOptions{})
	state.command("Copy")
	if got := driver.Clipboard(); got != "red\nred\nred" {
		t.Fatalf("multi-selection clipboard = %q", got)
	}
	state.command("Redact Selection")
	if doc.Editor.String() != "███ ███ ███" {
		t.Fatalf("multi-selection redaction = %q", doc.Editor.String())
	}
	state.command("Undo")
	if doc.Editor.String() != "red red red" || len(state.multiSelections[0].ranges) != 3 {
		t.Fatalf("multi-selection redaction undo = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Cut")
	if doc.Editor.String() != "  " || driver.Clipboard() != "red\nred\nred" {
		t.Fatalf("multi-selection cut = %q, clipboard=%q", doc.Editor.String(), driver.Clipboard())
	}
	state.command("Undo")
	if doc.Editor.String() != "red red red" {
		t.Fatalf("multi-selection cut undo = %q", doc.Editor.String())
	}
}

func TestMultiSelectionReadOnlyAndSplitCloneDoNotDuplicateEdits(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("one one")
	state.command("Clone to Other View")
	state.activatePane(0)
	clickID(t, driver, editorID)
	left.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectAll(document.SearchOptions{})
	state.readOnly[left] = true
	state.command("Cut")
	if left.Editor.String() != "one one" || len(state.multiSelections[0].ranges) != 2 {
		t.Fatal("read-only cut changed clone or selections")
	}
	delete(state.readOnly, left)
	driver.SetClipboard("Z")
	state.activatePane(1)
	left.Editor.MoveTo(0, false)
	state.command("Paste")
	if left.Editor.String() == "one one" {
		t.Fatal("active clone did not edit")
	}
	state.syncMultiSelection(0, left)
	if state.multiSelections[0].active || len(state.multiSelections[0].ranges) != 0 {
		t.Fatalf("other pane retained stale ranges: %+v", state.multiSelections[0])
	}
}

func TestMultiSelectionNativeCopyCutUsesDisjointClipboard(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("red red")
	clickID(t, driver, editorID)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectAll(document.SearchOptions{})
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(primary(), 'c'); err != nil || !handled || driver.Clipboard() != "red\nred" {
		t.Fatalf("native multi-copy handled=%v err=%v clipboard=%q", handled, err, driver.Clipboard())
	}
	if handled, err := driver.ShortcutChord(primary(), 'x'); err != nil || !handled || doc.Editor.String() != " " || driver.Clipboard() != "red\nred" {
		t.Fatalf("native multi-cut handled=%v err=%v text=%q clipboard=%q", handled, err, doc.Editor.String(), driver.Clipboard())
	}
}

func TestMultiSelectionPointerAndSelectAllDismissRanges(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectAll(document.SearchOptions{})
	clickID(t, driver, editorID)
	if state.multiSelections[0].active {
		t.Fatal("pointer click retained disjoint selections")
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectAll(document.SearchOptions{})
	state.command("Select All")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.multiSelections[0].active || doc.Editor.Selection() != (text.Range{Start: 0, End: 7}) {
		t.Fatalf("ordinary Select All was overridden: %+v, selection=%+v", state.multiSelections[0], doc.Editor.Selection())
	}
}

func TestMultiSelectionKeepsPaneLocalMainSelectionAcrossCloneFocus(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat cat cat")
	state.command("Clone to Other View")
	state.activatePane(0)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectNext(document.SearchOptions{})
	if len(state.multiSelections[0].ranges) != 2 {
		t.Fatal("left view did not add the second selection")
	}
	state.activatePane(1)
	doc.Editor.SetSelection(text.Range{Start: 8, End: 11})
	state.multiSelectNext(document.SearchOptions{})
	state.activatePane(0)
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || len(state.multiSelections[0].ranges) != 2 || len(state.multiSelections[1].ranges) != 2 {
		t.Fatalf("clone focus lost selection: left=%+v right=%+v main=%+v", state.multiSelections[0], state.multiSelections[1], doc.Editor.Selection())
	}
}

func TestMultiSelectionCollapsedRangesPaintAdditionalCarets(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a a")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	state.multiSelectAll(document.SearchOptions{})
	if changed, err := state.editMultiSelection(0, "X", multiInsert); err != nil || !changed {
		t.Fatalf("multi-caret insertion = %v, %v", changed, err)
	}
	snapshot := components.CodeEditorSnapshot{}
	state.paintMultiSelection(&snapshot, doc, 0)
	if !snapshot.ShowCaret || len(snapshot.SelectionRanges) != 0 || len(snapshot.ExtraCarets) != 1 ||
		snapshot.ExtraCarets[0].Offset != doc.Editor.ByteOffset(1) || snapshot.Caret != doc.Editor.ByteOffset(3) {
		t.Fatalf("collapsed multi-caret paint = %+v", snapshot)
	}
}

func TestMultiSelectionMenuPasteAppliesSameMultilineTextAtEachRange(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("α α")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	doc.Editor.ClearHistory()
	state.multiSelectAll(document.SearchOptions{})
	driver.SetClipboard("β\nγ")
	state.command("Paste")
	if doc.Editor.String() != "β\nγ β\nγ" || len(state.multiSelections[0].ranges) != 2 {
		t.Fatalf("multi-paste = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Undo")
	if doc.Editor.String() != "α α" || doc.Editor.CanUndo() {
		t.Fatalf("multi-paste undo = %q", doc.Editor.String())
	}
}

func TestMultiSelectionForwardDeleteIsOneUndoUnit(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("aX bX")
	doc.Editor.MoveTo(4, false)
	doc.Editor.ClearHistory()
	state.multiSelections[0] = multiSelectionState{
		doc: doc, value: doc.Editor.String(), active: true,
		ranges: []text.Range{{Start: 1, End: 1}, {Start: 4, End: 4}},
	}
	if changed, err := state.editMultiSelection(0, "", multiDeleteForward); err != nil || !changed || doc.Editor.String() != "a b" {
		t.Fatalf("forward delete at two carets = %q, %v", doc.Editor.String(), err)
	}
	state.command("Undo")
	if doc.Editor.String() != "aX bX" || doc.Editor.CanUndo() || !reflect.DeepEqual(state.multiSelections[0].ranges,
		[]text.Range{{Start: 1, End: 1}, {Start: 4, End: 4}}) {
		t.Fatalf("forward delete undo = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
}
