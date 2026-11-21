package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestMoveCurrentLineThroughCompactLineMenu(t *testing.T) {
	for _, tc := range []struct {
		command string
		caret   int
		moved   text.Range
	}{
		{"Move Up Current Line", 5, text.Range{Start: 2, End: 4}},
		{"Move Down Current Line", 3, text.Range{Start: 4, End: 7}},
	} {
		t.Run(tc.command, func(t *testing.T) {
			state, driver := startDriver(t)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			const original = "a\nbb\nc\nd"
			doc.Editor.Insert(original)
			doc.Editor.ClearHistory()
			clickID(t, driver, editorID)
			doc.Editor.MoveTo(tc.caret, false)
			choose := func() {
				clickID(t, driver, menuScope.At(1))
				clickID(t, driver, menuItemScope.At(10))
				for step := 0; step < len(lineCommands); step++ {
					var next gfx.ID
					for _, hit := range driver.Snapshot().Hits {
						if hit.Label == tc.command && hit.Role == gfx.RoleMenuItem {
							clickID(t, driver, hit.ID)
							return
						}
						if hit.ID == lineNextID {
							next = hit.ID
						}
					}
					if next.IsZero() {
						break
					}
					clickID(t, driver, next)
				}
				t.Fatalf("compact Line Operations is missing %q", tc.command)
			}
			choose()
			requireEditorCommandFocus(t, driver, editorID)
			if doc.Editor.String() != "a\nc\nbb\nd" || doc.Editor.Selection() != tc.moved || doc.Editor.Caret() != tc.moved.Start {
				t.Fatalf("move = %q, selection=%+v, caret=%d", doc.Editor.String(), doc.Editor.Selection(), doc.Editor.Caret())
			}
			if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != original || doc.Editor.CanUndo() {
				t.Fatalf("one Undo did not restore source: handled=%v text=%q err=%v", handled, doc.Editor.String(), err)
			}
			state.command("Read-Only on Current Document")
			choose()
			if doc.Editor.String() != original || doc.Editor.CanUndo() || state.notice != "Read-only document" {
				t.Fatalf("read-only move changed source: text=%q notice=%q", doc.Editor.String(), state.notice)
			}
		})
	}
}
