package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestPreferencesDismissalReturnsFocusToTheActiveEditor(t *testing.T) {
	for _, dismissal := range []string{"Escape", "Close"} {
		for _, pane := range []string{"left", "right"} {
			t.Run(dismissal+"/"+pane, func(t *testing.T) {
				state, driver := startDriver(t)
				if err := driver.Resize(640, 400); err != nil {
					t.Fatal(err)
				}
				left, right := state.space.Current(), state.space.NewDocument()
				left.Editor.Insert("alpha")
				right.Editor.Insert("bravo")
				left.Editor.ClearHistory()
				right.Editor.ClearHistory()
				state.command("Split View")
				state.paneDocs[0], state.paneDocs[1] = left, right
				state.paneOwner[left], state.paneOwner[right] = 0, 1
				state.invalidator.Invalidate()
				if err := driver.Pump(); err != nil {
					t.Fatal(err)
				}
				active, other, id := left, right, editorID
				if pane == "right" {
					active, other, id = right, left, rightEditorID
				}
				clickID(t, driver, id)
				active.Editor.SetSelection(text.Range{Start: 4, End: 1})
				original, untouched := active.Editor.String(), other.Editor.String()
				clickID(t, driver, menuScope.At(6))
				clickID(t, driver, menuItemScope.At(0))
				if !state.preferencesOpen || state.space.Current() != active {
					t.Fatal("Preferences did not open from the chosen pane")
				}
				clickID(t, driver, preferencesViewID)
				if dismissal == "Close" {
					clickID(t, driver, preferencesCloseID)
				} else if err := driver.Key(gfx.KeyEscape, 0); err != nil {
					t.Fatal(err)
				}
				if state.preferencesOpen || state.space.Current() != active || active.Editor.Caret() != 1 ||
					active.Editor.Selection() != (text.Range{Start: 1, End: 4}) || active.Editor.CanUndo() {
					t.Fatal("dismissal changed the pane, selection or history")
				}
				if err := driver.Type("x"); err != nil {
					t.Fatal(err)
				}
				if got := active.Editor.String(); got != original[:1]+"x"+original[4:] || other.Editor.String() != untouched {
					t.Fatalf("first text after dismissal: active=%q other=%q", got, other.Editor.String())
				}
				requireEditorCommandFocus(t, driver, id)
				if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || active.Editor.String() != original || active.Editor.CanUndo() {
					t.Fatalf("one Undo should restore the original text: handled=%v text=%q err=%v", handled, active.Editor.String(), err)
				}
			})
		}
	}
}
