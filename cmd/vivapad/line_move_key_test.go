package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestNotepadLineMoveChordsStayInTheActiveEditor(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   gfx.Key
		caret int
		moved text.Range
	}{
		{"up", gfx.KeyUp, 5, text.Range{Start: 2, End: 4}},
		{"down", gfx.KeyDown, 3, text.Range{Start: 4, End: 7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, driver := startDriver(t)
			left, right := state.space.Current(), state.space.NewDocument()
			const original = "a\nbb\nc\nd"
			left.Editor.Insert("other pane")
			right.Editor.Insert(original)
			right.Editor.ClearHistory()
			state.command("Split View")
			state.paneDocs[0], state.paneDocs[1], state.paneOwner[right] = left, right, 1
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, rightEditorID)
			press := func(modifiers gfx.Modifiers) {
				t.Helper()
				right.Editor.MoveTo(tc.caret, false)
				if err := driver.Key(tc.key, modifiers); err != nil {
					t.Fatal(err)
				}
			}
			press(gfx.ModifierShift)
			if right.Editor.Selection().Start == right.Editor.Selection().End || right.Editor.String() != original || right.Editor.CanUndo() {
				t.Fatal("Shift+arrow no longer extends the selection without editing")
			}
			const originalChord = gfx.ModifierControl | gfx.ModifierShift
			for _, modifiers := range []gfx.Modifiers{originalChord, gfx.ModifierAlt} {
				press(modifiers)
				if right.Editor.String() != "a\nc\nbb\nd" || right.Editor.Selection() != tc.moved || right.Editor.Caret() != tc.moved.Start || left.Editor.String() != "other pane" {
					t.Fatalf("line shortcut with modifiers %v: right=%q selection=%+v left=%q", modifiers, right.Editor.String(), right.Editor.Selection(), left.Editor.String())
				}
				if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || right.Editor.String() != original || right.Editor.CanUndo() {
					t.Fatalf("one Undo did not restore source: handled=%v text=%q err=%v", handled, right.Editor.String(), err)
				}
			}
			state.command("Read-Only on Current Document")
			press(originalChord)
			if state.notice != "Read-only document" || right.Editor.String() != original || right.Editor.CanUndo() {
				t.Fatalf("read-only move: text=%q notice=%q", right.Editor.String(), state.notice)
			}
			state.command("Read-Only on Current Document")
			clickID(t, driver, menuScope.At(6))
			clickID(t, driver, menuItemScope.At(0))
			press(originalChord)
			if !state.preferencesOpen || right.Editor.String() != original || right.Editor.CanUndo() {
				t.Fatal("line shortcut bypassed Preferences")
			}
		})
	}
}
