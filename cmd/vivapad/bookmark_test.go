package main

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/macro"
)

func TestBookmarksFollowInsertionsAndMergeDeletedLineBoundaries(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo\nthree\n")
	doc.GoToLine(3)
	state.command("Toggle Bookmark")
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("initial bookmarks = %v", got)
	}
	doc.Editor.MoveTo(0, false)
	doc.Editor.Insert("zero\n")
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{3}) {
		t.Fatalf("bookmark after insertion = %v", got)
	}
	doc.GoToLine(4)
	doc.DeleteLine()
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{3}) {
		t.Fatalf("deleted boundary did not merge bookmark %v", got)
	}
}

func TestBookmarkF2NavigationAndToggleThroughDriver(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo\nthree\nfour")
	clickID(t, driver, editorID)
	doc.GoToLine(2)
	if err := driver.Key(gfx.KeyF2, gfx.ModifierControl); err != nil {
		t.Fatal(err)
	}
	doc.GoToLine(4)
	if err := driver.Key(gfx.KeyF2, gfx.ModifierControl); err != nil {
		t.Fatal(err)
	}
	doc.GoToLine(1)
	if err := driver.Key(gfx.KeyF2, 0); err != nil {
		t.Fatal(err)
	}
	if got := bookmarkLine(doc); got != 1 {
		t.Fatalf("next bookmark moved to line %d, want 1", got)
	}
	if err := driver.Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if got := bookmarkLine(doc); got != 3 {
		t.Fatalf("previous bookmark did not wrap to line 3: %d", got)
	}
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("bookmarks = %v", got)
	}
}

func TestBookmarkPaintsInEditorGutter(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	marker := gfx.RGB(43, 104, 187)
	for _, primitive := range driver.Snapshot().Primitives {
		if primitive.Color == marker && primitive.Bounds.Width >= 6 && primitive.Bounds.Width <= 12 && primitive.Bounds.Height == primitive.Bounds.Width {
			return
		}
	}
	t.Fatal("bookmark marker was not painted")
}

func TestBookmarkMenuTogglesAndClearsCurrentDocument(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo")
	doc.GoToLine(2)
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Bookmark"))
	clickID(t, driver, bookmarkScope.At(0))
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("menu toggle bookmarks = %v", got)
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Bookmark"))
	clickID(t, driver, bookmarkScope.At(3))
	if got := state.bookmarkLines(doc); len(got) != 0 {
		t.Fatalf("menu clear bookmarks = %v", got)
	}
}

func TestBookmarkedLinesCopyCutAndUndo(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\n二\nthree")
	doc.SetLineEnding(document.CRLF)
	for _, line := range []int{1, 3} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	state.command("Copy Bookmarked Lines")
	if got := driver.Clipboard(); got != "one\r\nthree" || doc.Editor.String() != "one\n二\nthree" {
		t.Fatalf("copy = %q; document = %q", got, doc.Editor.String())
	}
	state.command("Cut Bookmarked Lines")
	if got := doc.Editor.String(); got != "二\n" || len(state.bookmarkLines(doc)) != 0 {
		t.Fatalf("cut result = %q, marks = %v", got, state.bookmarkLines(doc))
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "one\n二\nthree" {
		t.Fatalf("cut must undo in one step: %q", doc.Editor.String())
	}
}

func TestPasteToBookmarkedLinesReplacesEachWithoutLosingMarkers(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("a\nb\nc")
	for _, line := range []int{1, 3} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	driver.SetClipboard("X\r\nY")
	state.command("Paste to (Replace) Bookmarked Lines")
	if got := doc.Editor.String(); got != "X\nY\nb\nX\nY" {
		t.Fatalf("pasted marked lines = %q", got)
	}
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{1, 4}) {
		t.Fatalf("bookmarks shifted to %v", got)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\nb\nc" {
		t.Fatalf("paste must undo in one step: %q", doc.Editor.String())
	}
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("undo did not restore surviving bookmark positions: %v", got)
	}
	if !doc.Editor.Redo() || doc.Editor.String() != "X\nY\nb\nX\nY" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1, 4}) {
		t.Fatalf("redo did not restore marked paste: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestRemoveAndInverseBookmarkedLinesPreserveTrailingNewline(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb\nc\n")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	doc.GoToLine(4)
	state.command("Toggle Bookmark")
	state.command("Remove Non-Bookmarked Lines")
	if got := doc.Editor.String(); got != "b\n" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0, 1}) {
		t.Fatalf("kept marked lines = %q, marks = %v", got, state.bookmarkLines(doc))
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\nb\nc\n" {
		t.Fatalf("remove must undo in one step: %q", doc.Editor.String())
	}
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("undo moved surviving marks to %v", got)
	}
	state.command("Clear All Bookmarks")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	state.command("Inverse Bookmarks")
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{0, 2, 3}) {
		t.Fatalf("inverse bookmarks = %v", got)
	}
	state.command("Remove Bookmarked Lines")
	if got := doc.Editor.String(); got != "b\n" || len(state.bookmarkLines(doc)) != 0 {
		t.Fatalf("removed marked lines = %q, marks = %v", got, state.bookmarkLines(doc))
	}
}

func TestBookmarkFullMenuFitsCompactWindow(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("first\nsecond")
	doc.GoToLine(1)
	state.command("Toggle Bookmark")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Bookmark"))
	hits := driver.Snapshot().Hits
	visible := 0
	for _, hit := range hits {
		for index := range bookmarkCommands {
			if hit.ID != bookmarkScope.At(index) {
				continue
			}
			visible++
			if hit.Bounds.X < 0 || hit.Bounds.Y < menuHeight || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
				t.Fatalf("bookmark item %d outside compact window: %+v", index, hit.Bounds)
			}
		}
	}
	if visible != len(bookmarkCommands) {
		t.Fatalf("visible bookmark commands = %d, want %d", visible, len(bookmarkCommands))
	}
	clickID(t, driver, bookmarkScope.At(9))
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("inverse command from compact menu = %v", got)
	}
}

func TestBookmarkClipboardCommandsThroughMenu(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo")
	doc.GoToLine(1)
	state.command("Toggle Bookmark")
	choose := func(index int) {
		clickID(t, driver, menuScope.At(2))
		clickID(t, driver, searchCommandID(t, "Bookmark"))
		clickID(t, driver, bookmarkScope.At(index))
	}
	choose(5)
	if got := driver.Clipboard(); got != "one\n" || doc.Editor.String() != "one\ntwo" {
		t.Fatalf("menu copy = %q, text = %q", got, doc.Editor.String())
	}
	driver.SetClipboard("replaced")
	choose(6)
	if got := doc.Editor.String(); got != "replaced\ntwo" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("menu paste = %q, marks = %v", got, state.bookmarkLines(doc))
	}
	choose(4)
	if got := driver.Clipboard(); got != "replaced\n" || doc.Editor.String() != "two" {
		t.Fatalf("menu cut = %q, text = %q", got, doc.Editor.String())
	}
}

func TestDeleteLineMergesBookmarksWithoutRememberingDeletedSets(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame\nother")
	for _, line := range []int{1, 3} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	doc.GoToLine(1)
	state.command("Delete Line")
	if got := doc.Editor.String(); got != "same\nother" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0, 1}) {
		t.Fatalf("delete first duplicate = %q, marks = %v", got, state.bookmarkLines(doc))
	}
	if !doc.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1, 2}) {
		t.Fatalf("undo of duplicate delete = %q, marks = %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	if !doc.Editor.Redo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0, 1}) {
		t.Fatalf("redo of duplicate delete = %q, marks = %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	clickID(t, driver, editorID)
	doc.GoToLine(1)
	if _, err := driver.ShortcutChord(primary(), 'l'); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "other" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("shortcut duplicate delete = %q, marks = %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestDeleteSelectedLinesShiftsOnlySurvivingBookmarks(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb\nc\nd")
	for _, line := range []int{2, 3, 4} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	doc.Editor.SetSelection(text.Range{Start: 2, End: 6})
	state.command("Delete Line")
	if doc.Editor.String() != "a\nd" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("selected line deletion = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	if !doc.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{3}) {
		t.Fatalf("selected line undo = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestCodeEditorDeleteShortcutKeepsMergedLineMarker(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame")
	doc.GoToLine(1)
	state.command("Toggle Bookmark")
	clickID(t, driver, editorID)
	doc.GoToLine(1)
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'k'); err != nil || !handled {
		t.Fatalf("editor delete shortcut handled=%v, err=%v", handled, err)
	}
	if doc.Editor.String() != "same" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("editor shortcut transferred bookmark: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestNativeSelectionDeletionTracksIdenticalBookmarkedLines(t *testing.T) {
	for _, markedLine := range []int{1, 2} {
		t.Run(string(rune('0'+markedLine)), func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("same\nsame")
			doc.GoToLine(markedLine)
			state.command("Toggle Bookmark")
			clickID(t, driver, editorID)
			doc.Editor.SetSelection(text.Range{Start: 0, End: 5})
			if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
				t.Fatal(err)
			}
			if doc.Editor.String() != "same" {
				t.Fatalf("native delete = %q", doc.Editor.String())
			}
			got := state.bookmarkLines(doc)
			if !reflect.DeepEqual(got, []int{0}) {
				t.Fatalf("native delete transferred bookmark: marked line %d, got %v", markedLine, got)
			}
			if !doc.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
				t.Fatalf("native undo did not restore marker position: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
			}
			if !doc.Editor.Redo() {
				t.Fatal("native deletion could not be redone")
			}
			got = state.bookmarkLines(doc)
			if !reflect.DeepEqual(got, []int{0}) {
				t.Fatalf("native redo transferred bookmark: %v", got)
			}
		})
	}
}

func TestToolbarCutAndPasteKeepDuplicateLineBookmarkIdentity(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 5})
	clickID(t, driver, cutID)
	if doc.Editor.String() != "same" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("toolbar cut = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	driver.SetClipboard("new\n")
	doc.Editor.MoveTo(0, false)
	clickID(t, driver, pasteID)
	if doc.Editor.String() != "new\nsame" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("toolbar paste = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestDuplicateLineKeepsOriginalBookmarksOnRepeatedText(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame\nend")
	for _, line := range []int{1, 2} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	doc.GoToLine(1)
	state.command("Duplicate Line")
	if doc.Editor.String() != "same\nsame\nsame\nend" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0, 2}) {
		t.Fatalf("duplicated line bookmarks = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	if !doc.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0, 1}) {
		t.Fatalf("duplicate undo = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	if !doc.Editor.Redo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0, 2}) {
		t.Fatalf("duplicate redo = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestCodeEditorMoveAndDuplicateShortcutsTrackRepeatedLines(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame\nend")
	doc.GoToLine(1)
	state.command("Toggle Bookmark")
	clickID(t, driver, editorID)
	doc.GoToLine(1)
	if err := driver.Key(gfx.KeyDown, gfx.ModifierAlt); err != nil {
		t.Fatal(err)
	}
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("move down changed marker identity: %q, %v", doc.Editor.String(), got)
	}
	if err := driver.Key(gfx.KeyUp, gfx.ModifierAlt); err != nil {
		t.Fatal(err)
	}
	if got := state.bookmarkLines(doc); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("move up changed marker identity: %q, %v", doc.Editor.String(), got)
	}
	if err := duplicateDownShortcut(t, driver); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "same\nsame\nsame\nend" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{2}) {
		t.Fatalf("duplicate down copied marker unexpectedly: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

// duplicateDownShortcut presses the keyboard gesture that duplicates the current
// line down. macOS keeps Option+Shift+Down for the editor shortcut until column
// mode is armed; Windows and Linux start a rectangle selection directly with
// Alt+Shift arrows (docs/specs/001-vivapad/spec.md, Begin/End Select in Column
// Mode), so they duplicate through the Ctrl+D shortcut instead.
func duplicateDownShortcut(t *testing.T, driver *app.Driver) error {
	t.Helper()
	if runtime.GOOS == "darwin" {
		return driver.Key(gfx.KeyDown, gfx.ModifierAlt|gfx.ModifierShift)
	}
	handled, err := driver.ShortcutChord(primary(), 'd')
	if err != nil {
		return err
	}
	if !handled {
		t.Fatal("duplicate-line shortcut was not handled")
	}
	return nil
}

func TestSelectedLineMoveAndDuplicatePreserveMarkerOwnership(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb\nc\nd\ne")
	for _, line := range []int{2, 4} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	doc.Editor.SetSelection(text.Range{Start: 2, End: 6})
	state.applyCodeCommand(doc, components.CodeEditorMoveLinesDown)
	if doc.Editor.String() != "a\nd\nb\nc\ne" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("move selected lines = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	if !doc.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{3}) {
		t.Fatalf("move undo = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	state.applyCodeCommand(doc, components.CodeEditorDuplicateLinesUp)
	if doc.Editor.String() != "a\nb\nc\nb\nc\nd\ne" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{5}) {
		t.Fatalf("duplicate selected lines above = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestMovingBoundaryLineDoesNotMoveItsBookmark(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("first\nlast")
	doc.GoToLine(1)
	state.command("Toggle Bookmark")
	state.applyCodeCommand(doc, components.CodeEditorMoveLinesUp)
	if doc.Editor.String() != "first\nlast" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("top boundary move changed marks: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	doc.GoToLine(2)
	state.applyCodeCommand(doc, components.CodeEditorMoveLinesDown)
	if doc.Editor.String() != "first\nlast" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("bottom boundary move changed marks: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestMacroReplayPreservesBookmarksOnRepeatedLineDuplicate(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame\nend")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	doc.GoToLine(1)
	state.macroLast = []macro.Step{{Kind: macro.LineStep, Line: "Duplicate Line"}}
	state.playMacro(2)
	if doc.Editor.String() != "same\nsame\nsame\nsame\nend" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{3}) {
		t.Fatalf("macro duplicate = %q, marks %v, notice %q", doc.Editor.String(), state.bookmarkLines(doc), state.notice)
	}
	if !doc.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("macro undo = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	if !doc.Editor.Redo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{3}) {
		t.Fatalf("macro redo = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestMacroReplayMoveOfIdenticalLinesUsesOriginalMarkerEffects(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame\nend")
	doc.GoToLine(1)
	state.command("Toggle Bookmark")
	state.macroLast = []macro.Step{{Kind: macro.CodeStep, Code: components.CodeEditorMoveLinesDown}}
	state.playMacro(1)
	if doc.Editor.String() != "same\nsame\nend" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("macro move = %q, marks %v, notice %q", doc.Editor.String(), state.bookmarkLines(doc), state.notice)
	}
}

func TestMacroReplayTracksBookmarksAcrossTextAndLineSteps(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	doc.GoToLine(1)
	state.macroLast = []macro.Step{
		{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "new\n"},
		{Kind: macro.LineStep, Line: "Duplicate Line"},
	}
	state.playMacro(1)
	if doc.Editor.String() != "new\nsame\nsame\nsame" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{3}) {
		t.Fatalf("mixed macro = %q, marks %v, notice %q", doc.Editor.String(), state.bookmarkLines(doc), state.notice)
	}
}

func TestFailedMacroReplayLeavesBookmarksAndTextUntouched(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	state.macroLast = []macro.Step{{Kind: macro.LineStep, Line: "unknown"}}
	state.playMacro(1)
	if doc.Editor.String() != "same\nsame" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("failed macro changed document: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestLineSortProjectsSourceMarkerScopeWithoutSnapshotUndo(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("outside\nb\na\nb\noutside")
	for _, line := range []int{2, 5} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	doc.Editor.SetSelection(text.Range{Start: 8, End: 13})
	state.command("Sort Lines Ascending")
	if doc.Editor.String() != "outside\na\nb\nb\noutside" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{4}) {
		t.Fatalf("sorted bookmarks = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	if !doc.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{4}) {
		t.Fatalf("sort undo = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestRemoveDuplicateAndEmptyLinesDistinguishesRepeatedBookmarks(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("outside\nb\na\nb\noutside")
	for _, line := range []int{2, 4, 5} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	doc.Editor.SetSelection(text.Range{Start: 8, End: 13})
	state.command("Remove Duplicate Lines")
	if doc.Editor.String() != "outside\nb\na\noutside" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{3}) {
		t.Fatalf("deduplicated bookmarks = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: doc.Editor.RuneCount()}, "x\n\nx\n")
	state.command("Clear All Bookmarks")
	doc.GoToLine(3)
	state.command("Toggle Bookmark")
	state.command("Remove Empty Lines")
	if doc.Editor.String() != "x\nx" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("removed empty-line bookmarks = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestSortingSelectionProjectsSourceMarkerMergeAtOutsideBoundary(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame\nZ\nsame")
	for _, line := range []int{1, 2, 4} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	doc.Editor.SetSelection(text.Range{Start: 5, End: 11})
	state.command("Sort Lines Ascending")
	if doc.Editor.String() != "same\nZ\nsame\nsame" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0, 3}) {
		t.Fatalf("selected sort stole outside bookmark: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestRemoveBlankLinesMergesSourceTrailingLineBookmark(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert(" \n\nx\n")
	for _, line := range []int{3, 4} {
		doc.GoToLine(line)
		state.command("Toggle Bookmark")
	}
	state.command("Remove Blank Lines")
	if doc.Editor.String() != "x" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("removed blank lines changed trailing mark: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestReplaceCurrentMergesMarkersFromEitherSideOfDeletedLF(t *testing.T) {
	for _, markedLine := range []int{1, 2} {
		t.Run(string(rune('0'+markedLine)), func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("same\nsame\nend")
			doc.GoToLine(markedLine)
			state.command("Toggle Bookmark")
			doc.Editor.SetSelection(text.Range{Start: 0, End: 5})
			state.query.Insert("same\n")
			clickID(t, driver, searchID)
			clickID(t, driver, replaceID)
			if doc.Editor.String() != "same\nend" {
				t.Fatalf("current replacement = %q", doc.Editor.String())
			}
			marks := state.bookmarkLines(doc)
			if !reflect.DeepEqual(marks, []int{0}) {
				t.Fatalf("replacement transferred bookmark from line %d: %v", markedLine, marks)
			}
		})
	}
}

func TestReplaceAllKeepsBookmarkOnOriginalLineAfterInsertedNewlines(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	state.query.Insert("same")
	state.replacement.Insert("same\n")
	clickID(t, driver, searchID)
	clickID(t, driver, replaceAllID)
	if doc.Editor.String() != "same\n\nsame\n" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{3}) {
		t.Fatalf("replace-all bookmarks = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	if !doc.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("replace-all undo = %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestReplaceOpenTracksEachDocumentsBookmarksAndUndo(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Editor.Insert("same\nsame")
	first.GoToLine(2)
	state.command("Toggle Bookmark")
	clickID(t, driver, newID)
	second := state.space.Current()
	second.Editor.Insert("same\nsame")
	second.GoToLine(1)
	state.command("Toggle Bookmark")
	state.query.Insert("same")
	state.replacement.Insert("same\n")
	clickID(t, driver, searchID)
	clickID(t, driver, replaceOpenID)
	if first.Editor.String() != "same\n\nsame\n" || second.Editor.String() != "same\n\nsame\n" ||
		!reflect.DeepEqual(state.bookmarkLines(first), []int{3}) || !reflect.DeepEqual(state.bookmarkLines(second), []int{1}) {
		t.Fatalf("replace open marks = first %q %v, second %q %v", first.Editor.String(), state.bookmarkLines(first), second.Editor.String(), state.bookmarkLines(second))
	}
	if !first.Editor.Undo() || !second.Editor.Undo() || !reflect.DeepEqual(state.bookmarkLines(first), []int{1}) || !reflect.DeepEqual(state.bookmarkLines(second), []int{0}) {
		t.Fatalf("replace open undo marks = first %v, second %v", state.bookmarkLines(first), state.bookmarkLines(second))
	}
}

func TestReplacementSpanningNewlineKeepsMergedMarkerBeforeInsertedLF(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("head\nsame\nend")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	state.query.Insert("\nsame")
	state.replacement.Insert("\nnew")
	clickID(t, driver, searchID)
	clickID(t, driver, replaceAllID)
	if doc.Editor.String() != "head\nnew\nend" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("multiline replacement lost marker: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestInvalidReplaceOpenKeepsExistingBookmarks(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("same\nsame")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	state.query.Insert("(")
	state.replacement.Insert("new")
	state.searchOptions.Regex = true
	clickID(t, driver, searchID)
	clickID(t, driver, replaceOpenID)
	if doc.Editor.String() != "same\nsame" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("invalid batch replacement changed marker: %q, %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestRegexCaptureReplacementKeepsUnicodeBookmarks(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("猫1\n猫2")
	doc.GoToLine(2)
	state.command("Toggle Bookmark")
	state.query.Insert(`(猫)(\d)`)
	state.replacement.Insert("$2-$1\n")
	state.searchOptions.Regex = true
	clickID(t, driver, searchID)
	clickID(t, driver, replaceAllID)
	if doc.Editor.String() != "1-猫\n\n2-猫\n" || !reflect.DeepEqual(state.bookmarkLines(doc), []int{3}) {
		t.Fatalf("unicode regex replacement = %q, marks %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}
