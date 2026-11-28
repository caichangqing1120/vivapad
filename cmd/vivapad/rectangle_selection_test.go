package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestColumnSelectionMenuAndColumnEditorReplaceSelectedRows(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	before := "abcde\nxy\n猫123\nlast"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select in Column Mode")
	if !state.menuSelected("Begin/End Select in Column Mode") {
		t.Fatal("column selection anchor did not start")
	}
	state.command("Begin/End Select")
	if state.menuSelected("Begin/End Select in Column Mode") || !state.menuSelected("Begin/End Select") {
		t.Fatal("stream and column anchors stayed active together")
	}
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(12, false)
	state.command("Begin/End Select in Column Mode")
	if !state.rectangles[0].active || state.menuSelected("Begin/End Select in Column Mode") {
		t.Fatal("column rectangle was not completed")
	}
	state.command("Column Editor...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, columnTextID)
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, columnApplyID)
	if doc.Editor.String() != "abXe\nxyX\n猫X3\nlast" || state.rectangles[0].active {
		t.Fatalf("column rectangle edited %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("column rectangle Undo = %q", doc.Editor.String())
	}
}

func TestColumnSelectionAltShiftNavigationAndSplitIsolation(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("ab\ncd\nef")
	left.Editor.MoveTo(1, false)
	state.command("Split View")
	state.activatePane(1)
	right := state.space.NewDocument()
	state.showInActivePane(right)
	right.Editor.Insert("x\ny\nz")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	right.Editor.MoveTo(0, false)
	state.command("Begin/End Select in Column Mode")
	if err := driver.Key(gfx.KeyDown, gfx.ModifierAlt|gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyRight, gfx.ModifierAlt|gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if !state.rectangles[1].active || state.rectangles[0].active || left.Editor.String() != "ab\ncd\nef" {
		t.Fatal("Alt+Shift rectangle escaped active view")
	}
	rows := right.RectangleRows(state.rectangleForPane(1))
	if len(rows) != 2 || rows[0].Line != 0 || rows[1].Line != 1 {
		t.Fatalf("Alt+Shift rows = %+v", rows)
	}
	state.activatePane(0)
	if state.rectangles[0].active || left.Editor.Selection() != (text.Range{Start: 1, End: 1}) {
		t.Fatal("split view inherited another pane's rectangle")
	}
}

// TestColumnArrowsFollowDocumentedPlatformPolicy pins the shortcut split the
// spec describes for Begin/End Select in Column Mode: until column mode is
// armed, macOS keeps Option+Shift for word navigation and the editor's duplicate
// shortcut, while Windows and Linux start a rectangle selection directly with
// Alt+Shift arrows.
func TestColumnArrowsFollowDocumentedPlatformPolicy(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd\nef")
	doc.Editor.MoveTo(1, false)
	clickID(t, driver, editorID)
	if err := driver.Key(gfx.KeyDown, gfx.ModifierAlt|gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		if state.rectangles[0].active {
			t.Fatal("Option+Shift+Down started a column selection before column mode was armed")
		}
		return
	}
	if !state.rectangles[0].active {
		t.Fatal("Alt+Shift+Down did not start a column selection")
	}
	if got := doc.Editor.String(); got != "ab\ncd\nef" {
		t.Fatalf("column selection edited the text: %q", got)
	}
}

func TestColumnSelectionShortcutArmsModeWithoutEditing(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abc\ndef")
	doc.Editor.MoveTo(1, false)
	if handled, err := driver.ShortcutChord(gfx.ModifierAlt|gfx.ModifierShift, 'b'); err != nil || !handled {
		t.Fatalf("column-mode shortcut handled=%v err=%v", handled, err)
	}
	if !state.menuSelected("Begin/End Select in Column Mode") || doc.Editor.String() != "abc\ndef" {
		t.Fatal("column-mode shortcut did not arm selection safely")
	}
}

func TestColumnSelectionReleasesClosedDocument(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(0, false)
	state.command("Begin/End Select in Column Mode")
	previous := state.rectangles[0]
	state.recordRectangleTransition(0, doc, "before", "after", previous)
	state.altEditorGesture = altEditorGesture{doc: doc, dragging: true}
	state.releaseDocument(doc)
	if state.rectangles[0].doc != nil || state.altEditorGesture.doc != nil || len(state.rectangleHistory[0]) != 0 || state.rectangleHistoryIndex[0] != 0 {
		t.Fatal("closed document retained a rectangular selection")
	}
}

func TestColumnSelectionHistoryIsBounded(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	for index := 0; index < 140; index++ {
		before := strconv.Itoa(index)
		state.recordRectangleTransition(0, doc, before, before+"x", rectangleSelection{doc: doc, active: true})
	}
	if len(state.rectangleHistory[0]) != 128 || state.rectangleHistoryIndex[0] != 128 {
		t.Fatalf("rectangle history grew to %d entries at %d", len(state.rectangleHistory[0]), state.rectangleHistoryIndex[0])
	}
}

func TestColumnSelectionAltDragKeepsVirtualColumnsWithoutEditing(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\nc\n")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	caret, ok := state.codeEditor().CaretBounds()
	if !ok {
		t.Fatal("caret bounds missing")
	}
	pointX, pointY := caret.X+95, caret.Y+caret.Height/2
	if err := driver.KeyDown(gfx.KeyAltLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := driver.Press(pointX, pointY, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(pointX, pointY+caret.Height); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(pointX, pointY+caret.Height, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.KeyUp(gfx.KeyAltLeft, ""); err != nil {
		t.Fatal(err)
	}
	selection := state.rectangles[0]
	if !selection.active || selection.anchorVirtual == 0 || selection.caretVirtual == 0 || doc.Editor.String() != "ab\nc\n" || doc.Editor.CanUndo() {
		t.Fatalf("Alt drag changed document or lost virtual rectangle: %+v", selection)
	}
	if rows := doc.RectangleRows(state.rectangleForPane(0)); len(rows) != 2 || rows[0].VirtualCells == 0 || rows[1].VirtualCells == 0 {
		t.Fatalf("Alt drag rows = %+v", rows)
	}
}

func TestColumnSelectionNativeTypingAndPasteStayOnEveryCaret(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"X", "Y"} {
		if err := driver.Type(value); err != nil {
			t.Fatal(err)
		}
	}
	driver.SetClipboard("!")
	if _, err := driver.ShortcutChord(primary(), 'v'); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "aXY!b\ncXY!d" || !state.rectangles[0].active {
		t.Fatalf("native column input = %q, rect=%+v", doc.Editor.String(), state.rectangles[0])
	}
	for _, want := range []string{"aXYb\ncXYd", "aXb\ncXd", "ab\ncd"} {
		if !doc.Editor.Undo() || doc.Editor.String() != want {
			t.Fatalf("column input Undo = %q, want %q", doc.Editor.String(), want)
		}
	}
}

func TestColumnSelectionBackspaceDeletesEveryCaret(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "b\nd" || !state.rectangles[0].active {
		t.Fatalf("column Backspace = %q, rect=%+v", doc.Editor.String(), state.rectangles[0])
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd" || doc.Editor.CanUndo() {
		t.Fatalf("column Backspace Undo = %q", doc.Editor.String())
	}
}

func TestColumnSelectionIMECommitsToEveryCaretAsOneUndo(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Compose("zhong"); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "ab\nczhongd" || !state.rectangles[0].active {
		t.Fatalf("IME preedit changed secondary carets: %q", doc.Editor.String())
	}
	if err := driver.Compose("中"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Compose(""); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "a中b\nc中d" || !state.rectangles[0].active {
		t.Fatalf("IME commit = %q, rect=%+v", doc.Editor.String(), state.rectangles[0])
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd" || doc.Editor.CanUndo() {
		t.Fatalf("IME column Undo = %q", doc.Editor.String())
	}
}

func TestColumnSelectionIMEInsertCommitUsesEveryCaret(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Compose("zhong"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("中"); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "a中b\nc中d" {
		t.Fatalf("IME insert commit = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd" || doc.Editor.CanUndo() {
		t.Fatalf("IME insert Undo = %q", doc.Editor.String())
	}
}

func TestColumnSelectionDeleteAndOrdinaryPointerDismiss(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyDelete, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "a\nc" || !state.rectangles[0].active {
		t.Fatalf("column Delete = %q", doc.Editor.String())
	}
	clickID(t, driver, editorID)
	if state.rectangles[0].active || doc.Editor.String() != "a\nc" {
		t.Fatal("ordinary click retained rectangle or edited text")
	}
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	state.command("Begin/End Select in Column Mode")
	state.showInActivePane(state.space.NewDocument())
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.rectangles[0].active {
		t.Fatal("tab change retained old document rectangle")
	}
}

func TestColumnSelectionMenuScrollsToLastCommandAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	if state.editMenuVisible >= len(menuCommands[1]) {
		t.Fatal("Edit menu did not constrain the visible rows")
	}
	clickID(t, driver, editMenuNextID)
	if state.editMenuOffset != 1 {
		t.Fatalf("Edit scroll offset = %d", state.editMenuOffset)
	}
	clickID(t, driver, menuItemScope.At(23))
	if !state.menuSelected("Begin/End Select in Column Mode") || state.menuOpen != -1 {
		t.Fatal("last Edit command was not reachable")
	}
}

func TestColumnSelectionMenuCopyAndCutPreserveRowsAndUndo(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abcde\nxy\n猫123\nlast")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(12, false)
	state.command("Begin/End Select in Column Mode")
	state.command("Copy")
	if got := driver.Clipboard(); got != "cd\n\n12\n" || doc.Editor.String() != "abcde\nxy\n猫123\nlast" || doc.Editor.CanUndo() {
		t.Fatalf("column Copy = %q, text=%q", got, doc.Editor.String())
	}
	state.command("Cut")
	if got := driver.Clipboard(); got != "cd\n\n12\n" || doc.Editor.String() != "abe\nxy\n猫3\nlast" || !state.rectangles[0].active {
		t.Fatalf("column Cut = %q, text=%q, rect=%+v", got, doc.Editor.String(), state.rectangles[0])
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "abcde\nxy\n猫123\nlast" || doc.Editor.CanUndo() {
		t.Fatalf("column Cut Undo = %q", doc.Editor.String())
	}
}

func TestColumnSelectionKeyboardCopyAndCut(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abcde\nxy\n猫123")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(12, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	driver.SetClipboard("sentinel")
	if handled, err := driver.ShortcutChord(primary(), 'c'); err != nil || !handled {
		t.Fatalf("column Copy shortcut handled=%v err=%v", handled, err)
	}
	if driver.Clipboard() != "cd\n\n12\n" || doc.Editor.String() != "abcde\nxy\n猫123" {
		t.Fatalf("column Copy shortcut = %q, text=%q", driver.Clipboard(), doc.Editor.String())
	}
	if handled, err := driver.ShortcutChord(primary(), 'x'); err != nil || !handled {
		t.Fatalf("column Cut shortcut handled=%v err=%v", handled, err)
	}
	if driver.Clipboard() != "cd\n\n12\n" || doc.Editor.String() != "abe\nxy\n猫3" {
		t.Fatalf("column Cut shortcut = %q, text=%q", driver.Clipboard(), doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "abcde\nxy\n猫123" || doc.Editor.CanUndo() {
		t.Fatalf("column Cut shortcut Undo = %q", doc.Editor.String())
	}
}

func TestColumnSelectionKeyboardCopyTracksChangedColumns(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abc\ndef")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(5, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.ShortcutChord(primary(), 'c'); err != nil || driver.Clipboard() != "" {
		t.Fatalf("zero-width copy = %q, %v", driver.Clipboard(), err)
	}
	if err := driver.Key(gfx.KeyRight, gfx.ModifierAlt|gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.ShortcutChord(primary(), 'c'); err != nil || driver.Clipboard() != "b\ne\n" {
		t.Fatalf("expanded copy = %q, %v", driver.Clipboard(), err)
	}
}

func TestColumnSelectionUndoRedoRestoresSuccessiveMultiCarets(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, letter := range []string{"X", "Y"} {
		if err := driver.Type(letter); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []struct {
		value, copied string
		column        int
	}{{"aXb\ncXd", "", 2}, {"ab\ncd", "", 1}} {
		if _, err := driver.ShortcutChord(primary(), 'z'); err != nil {
			t.Fatal(err)
		}
		rows := doc.RectangleRows(state.rectangleForPane(0))
		if doc.Editor.String() != want.value || !state.rectangles[0].active || len(rows) != 2 || rows[0].LeftColumn != want.column || rows[0].RightColumn != want.column {
			t.Fatalf("column Undo = %q, rows=%+v rect=%+v", doc.Editor.String(), rows, state.rectangles[0])
		}
	}
	for _, want := range []string{"aXb\ncXd", "aXYb\ncXYd"} {
		if _, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'z'); err != nil {
			t.Fatal(err)
		}
		if doc.Editor.String() != want || !state.rectangles[0].active {
			t.Fatalf("column Redo = %q, rect=%+v", doc.Editor.String(), state.rectangles[0])
		}
	}
}

func TestColumnSelectionUndoRestoresSelectedRectangle(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abc\nxyz")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("Q"); err != nil || doc.Editor.String() != "aQ\nxQ" {
		t.Fatalf("rectangle replacement = %q, %v", doc.Editor.String(), err)
	}
	if _, err := driver.ShortcutChord(primary(), 'z'); err != nil || doc.Editor.String() != "abc\nxyz" {
		t.Fatalf("rectangle replacement Undo = %q, %v", doc.Editor.String(), err)
	}
	if _, err := driver.ShortcutChord(primary(), 'c'); err != nil || driver.Clipboard() != "bc\r\nyz\r\n" && driver.Clipboard() != "bc\nyz\n" {
		t.Fatalf("restored rectangle Copy = %q, %v", driver.Clipboard(), err)
	}
}

func TestColumnSelectionUndoRedoAcrossMultilinePaste(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	driver.SetClipboard("X\nY\nZ")
	if _, err := driver.ShortcutChord(primary(), 'v'); err != nil || doc.Editor.String() != "aXb\ncYd\n Z" || state.rectangles[0].active {
		t.Fatalf("multiline paste = %q, %v", doc.Editor.String(), err)
	}
	if _, err := driver.ShortcutChord(primary(), 'z'); err != nil || doc.Editor.String() != "ab\ncd" || !state.rectangles[0].active {
		t.Fatalf("multiline Undo = %q rect=%+v err=%v", doc.Editor.String(), state.rectangles[0], err)
	}
	if _, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'z'); err != nil || doc.Editor.String() != "aXb\ncYd\n Z" || state.rectangles[0].active {
		t.Fatalf("multiline Redo = %q rect=%+v err=%v", doc.Editor.String(), state.rectangles[0], err)
	}
}

func TestColumnSelectionMenuCutUndoRedoRestoresRectangle(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abc\nxyz")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	state.command("Cut")
	if doc.Editor.String() != "a\nx" || !state.rectangles[0].active {
		t.Fatalf("column Cut = %q", doc.Editor.String())
	}
	state.command("Undo")
	if doc.Editor.String() != "abc\nxyz" || !state.rectangles[0].active {
		t.Fatalf("column menu Undo = %q", doc.Editor.String())
	}
	state.command("Copy")
	if driver.Clipboard() != "bc\nyz\n" {
		t.Fatalf("restored column copy = %q", driver.Clipboard())
	}
	state.command("Redo")
	if doc.Editor.String() != "a\nx" || !state.rectangles[0].active {
		t.Fatalf("column menu Redo = %q", doc.Editor.String())
	}
}

func TestColumnSelectionUndoFromSharedCloneRestoresOwningView(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	state.command("Clone to Other View")
	if !state.splitEnabled || state.paneDocument(0) != doc || state.paneDocument(1) != doc {
		t.Fatal("clone did not share the document")
	}
	state.activatePane(0)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("X"); err != nil || doc.Editor.String() != "aXb\ncXd" {
		t.Fatalf("clone rectangle typing = %q, %v", doc.Editor.String(), err)
	}
	state.activatePane(1)
	clickID(t, driver, rightEditorID)
	if _, err := driver.ShortcutChord(primary(), 'z'); err != nil || doc.Editor.String() != "ab\ncd" {
		t.Fatalf("clone Undo = %q, %v", doc.Editor.String(), err)
	}
	if !state.rectangles[0].active || state.rectangles[1].active || state.rectangleHistoryIndex[0] != 0 || state.activePane != 1 {
		t.Fatalf("clone Undo lost selection ownership: left=%+v right=%+v", state.rectangles[0], state.rectangles[1])
	}
	if _, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'z'); err != nil || doc.Editor.String() != "aXb\ncXd" {
		t.Fatalf("clone Redo = %q, %v", doc.Editor.String(), err)
	}
	if !state.rectangles[0].active || state.rectangles[1].active || state.rectangleHistoryIndex[0] != 1 || state.activePane != 1 {
		t.Fatalf("clone Redo moved selection ownership: left=%+v right=%+v", state.rectangles[0], state.rectangles[1])
	}
}

func TestColumnSelectionInterleavedCloneHistoryKeepsViewOwnership(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	state.command("Clone to Other View")
	state.activatePane(0)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	state.activatePane(1)
	clickID(t, driver, rightEditorID)
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(6, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("Y"); err != nil || doc.Editor.String() != "aXYb\ncXYd" {
		t.Fatalf("second clone edit = %q, %v", doc.Editor.String(), err)
	}
	state.command("Undo")
	if doc.Editor.String() != "aXb\ncXd" || !state.rectangles[1].active || state.rectangleHistoryIndex[1] != 0 {
		t.Fatalf("first clone Undo = %q, right=%+v", doc.Editor.String(), state.rectangles[1])
	}
	state.command("Undo")
	if doc.Editor.String() != "ab\ncd" || !state.rectangles[0].active || state.rectangleHistoryIndex[0] != 0 || state.activePane != 1 {
		t.Fatalf("second clone Undo = %q, left=%+v", doc.Editor.String(), state.rectangles[0])
	}
	state.command("Redo")
	state.command("Redo")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "aXYb\ncXYd" || !state.rectangles[0].active || !state.rectangles[1].active || state.rectangleHistoryIndex[0] != 1 || state.rectangleHistoryIndex[1] != 1 {
		t.Fatalf("clone Redo ownership = %q, left=%+v right=%+v", doc.Editor.String(), state.rectangles[0], state.rectangles[1])
	}
}

func TestColumnSelectionSharedCloneStructuralEditDropsStaleOtherView(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	state.command("Clone to Other View")
	state.activatePane(0)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.activatePane(1)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	driver.SetClipboard("X\nY\nZ")
	if _, err := driver.ShortcutChord(primary(), 'v'); err != nil {
		t.Fatal(err)
	}
	if state.rectangles[0].active || state.rectangles[1].active {
		t.Fatalf("structural edit retained stale rectangle: left=%+v right=%+v", state.rectangles[0], state.rectangles[1])
	}
}

func TestColumnSelectionKeyboardMultilinePasteExtendsExistingRows(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd\nef")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	driver.SetClipboard("X\nY\nZ")
	if handled, err := driver.ShortcutChord(primary(), 'v'); !handled || err != nil {
		t.Fatalf("column paste handled=%v err=%v", handled, err)
	}
	if doc.Editor.String() != "aXb\ncYd\neZf" || state.rectangles[0].active {
		t.Fatalf("multiline paste = %q rect=%+v", doc.Editor.String(), state.rectangles[0])
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd\nef" || doc.Editor.CanUndo() {
		t.Fatalf("multiline paste Undo = %q", doc.Editor.String())
	}
}

func TestColumnSelectionReadOnlyKeyboardCopyDoesNotCut(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abc\ndef")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	state.readOnly[doc] = true
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(primary(), 'c'); !handled || err != nil || driver.Clipboard() != "bc\nef\n" {
		t.Fatalf("read-only column Copy handled=%v err=%v clipboard=%q", handled, err, driver.Clipboard())
	}
	if _, err := driver.ShortcutChord(primary(), 'x'); err != nil || doc.Editor.String() != "abc\ndef" {
		t.Fatalf("read-only column Cut changed text: %q, %v", doc.Editor.String(), err)
	}
}

func TestRectangularClipboardPastesAtOrdinaryCaretByKeyboardAndMenu(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	if err := app.SetRectangularClipboardText("X\nY\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.ShortcutChord(primary(), 'v'); err != nil || doc.Editor.String() != "aXb\ncYd" {
		t.Fatalf("rectangular shortcut paste = %q, %v", doc.Editor.String(), err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd" || doc.Editor.CanUndo() {
		t.Fatalf("rectangular shortcut paste Undo = %q", doc.Editor.String())
	}
	doc.Editor.MoveTo(1, false)
	state.command("Paste")
	if doc.Editor.String() != "aXb\ncYd" {
		t.Fatalf("rectangular menu paste = %q", doc.Editor.String())
	}
	if err := app.SetClipboardText("M\nN"); err != nil {
		t.Fatal(err)
	}
	doc.Editor.Undo()
	doc.Editor.MoveTo(1, false)
	state.command("Paste")
	if doc.Editor.String() != "aM\nNb\ncd" {
		t.Fatalf("plain stream paste changed shape = %q", doc.Editor.String())
	}
}

func TestCaptureColumnSelectionWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RECTANGLE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RECTANGLE_SHOTS to capture column selection")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("abcde\nxy\n猫123\nlast")
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(12, false)
	state.command("Begin/End Select in Column Mode")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s rectangle layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "column-selection-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	clickID(t, session.Driver(), menuScope.At(1))
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("column menu top layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "column-selection-menu-top-compact.png")); err != nil {
		t.Fatal(err)
	}
	state.editMenuOffset = len(menuCommands[1]) - state.editMenuVisible
	state.invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("column menu bottom layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "column-selection-menu-bottom-compact.png")); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureTabBoundaryRectangleWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RECTANGLE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RECTANGLE_SHOTS to capture tab boundary selection")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\tb\nab")
	doc.Editor.MoveTo(6, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("tab rectangle layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "column-selection-tab-compact.png")); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRestoredRectangleWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RECTANGLE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RECTANGLE_SHOTS to capture restored selection")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("abc\nxyz")
	doc.Editor.ClearHistory()
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(7, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Type("Q"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Driver().ShortcutChord(primary(), 'z'); err != nil || !state.rectangles[0].active {
		t.Fatalf("rectangle Undo did not restore selection: %v", err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("restored rectangle layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "column-selection-undo-compact.png")); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureSharedCloneRectangleUndoWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RECTANGLE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RECTANGLE_SHOTS to capture shared-clone selection")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("abc\nxyz")
	state.command("Clone to Other View")
	state.activatePane(0)
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select in Column Mode")
	doc.Editor.MoveTo(6, false)
	state.command("Begin/End Select in Column Mode")
	state.invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Type("Q"); err != nil {
		t.Fatal(err)
	}
	state.activatePane(1)
	clickID(t, session.Driver(), rightEditorID)
	if _, err := session.Driver().ShortcutChord(primary(), 'z'); err != nil || !state.rectangles[0].active || state.rectangles[1].active {
		t.Fatalf("clone Undo did not restore left selection: %v", err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("shared-clone rectangle layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "column-selection-clone-undo-compact.png")); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRectangularClipboardPasteWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RECTANGLE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RECTANGLE_SHOTS to capture clipboard paste")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd\nef")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), editorID)
	doc.Editor.MoveTo(1, false)
	if err := app.SetRectangularClipboardText("X\nY\nZ\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Driver().ShortcutChord(primary(), 'v'); err != nil || doc.Editor.String() != "aXb\ncYd\neZf" {
		t.Fatalf("captured rectangle paste = %q, %v", doc.Editor.String(), err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("clipboard rectangle layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "column-clipboard-paste-compact.png")); err != nil {
		t.Fatal(err)
	}
}
