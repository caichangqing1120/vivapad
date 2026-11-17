package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestMatchingBraceCommandsHandleUnicodeAndSelectionEndpoints(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	value := "猫{a(b)c}尾"
	doc.Editor.Insert(value)
	open, close := strings.IndexByte(value, '{'), strings.IndexByte(value, '}')
	doc.Editor.MoveTo(doc.Editor.RuneIndex(open+1), false)
	state.command("Go to Matching Brace")
	if got := doc.Editor.Caret(); got != doc.Editor.RuneIndex(close) {
		t.Fatalf("matching-brace caret = %d, want %d", got, doc.Editor.RuneIndex(close))
	}
	state.command("Select All In-between {} [] or ()")
	want := text.Range{Start: doc.Editor.RuneIndex(open), End: doc.Editor.RuneIndex(close + 1)}
	if got := doc.Editor.Selection(); got != want {
		t.Fatalf("brace selection = %+v, want %+v", got, want)
	}
}

func TestMatchingBracePrefersBracketBeforeCaretAndPreservesUnmatchedSelection(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("{}()")
	doc.Editor.MoveTo(2, false)
	state.command("Go to Matching Brace")
	if got := doc.Editor.Caret(); got != 0 {
		t.Fatalf("between adjacent pairs moved to %d instead of earlier pair", got)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	doc.Editor.Insert("unclosed(")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	before := doc.Editor.Selection()
	state.command("Go to Matching Brace")
	state.command("Select All In-between {} [] or ()")
	if doc.Editor.Selection() != before || doc.Editor.String() != "unclosed(" {
		t.Fatalf("unmatched bracket changed selection or text: %+v, %q", doc.Editor.Selection(), doc.Editor.String())
	}
}

func TestMatchingBraceShortcutsAndCompactMenu(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("{猫}")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	if handled, err := driver.ShortcutChord(primary(), 'b'); err != nil || !handled {
		t.Fatalf("go to brace shortcut handled=%v err=%v", handled, err)
	}
	if doc.Editor.Caret() != 2 {
		t.Fatalf("shortcut caret = %d, want 2", doc.Editor.Caret())
	}
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierAlt, 'b'); err != nil || !handled {
		t.Fatalf("select brace shortcut handled=%v err=%v", handled, err)
	}
	if doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) {
		t.Fatalf("shortcut selection = %+v", doc.Editor.Selection())
	}
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(1, false)
	clickID(t, driver, menuScope.At(2))
	for _, index := range []int{13, 14} {
		bounds := revealSearchMenuItem(t, driver, index)
		if bounds.Y+bounds.Height > 400-statusHeight {
			t.Fatalf("brace menu item %d is clipped: %+v", index, bounds)
		}
	}
	clickID(t, driver, menuItemScope.At(13))
	if doc.Editor.Caret() != 2 {
		t.Fatalf("menu go-to caret = %d", doc.Editor.Caret())
	}
}

func TestMatchingBraceShortcutUsesFocusedSplitPane(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left {x}")
	right := state.space.NewDocument()
	right.Editor.Insert("right [猫]")
	state.toggleSplit()
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(1)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, rightEditorID)
	value := right.Editor.String()
	opening, closing := strings.IndexByte(value, '['), strings.IndexByte(value, ']')
	right.Editor.MoveTo(right.Editor.RuneIndex(opening+1), false)
	leftBefore := left.Editor.Selection()
	if handled, err := driver.ShortcutChord(primary(), 'b'); err != nil || !handled {
		t.Fatalf("split shortcut handled=%v err=%v", handled, err)
	}
	if state.space.Current() != right || right.Editor.Caret() != right.Editor.RuneIndex(closing) || left.Editor.Selection() != leftBefore {
		t.Fatalf("split match changed wrong pane: active=%p right=%+v left=%+v", state.space.Current(), right.Editor.Selection(), left.Editor.Selection())
	}
}
