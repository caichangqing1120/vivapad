package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestPathCompletionAtInteriorCaret(t *testing.T) {
	for _, tc := range []struct {
		name, typed, suffix string
		noOp                bool
	}{
		{"existing file", "al", "pha.txt", true},
		{"unfinished final component", "al", "-old.txt", false},
		{"exact filename prefix", "alpha.txt", "-old", false},
		{"interior directory component", "al", "/child.txt", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "alpha.txt"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			prefix := `open "` + tc.typed
			before := prefix + tc.suffix + `" tail`
			file := filepath.Join(root, "buffer.txt")
			if err := os.WriteFile(file, []byte(before), 0600); err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{file})
			doc := state.space.Current()
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			clickID(t, driver, editorID)
			doc.Editor.MoveTo(len(prefix), false)
			if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, ' '); err != nil || !handled {
				t.Fatalf("path shortcut handled=%v err=%v", handled, err)
			}
			if tc.noOp {
				if state.wordCompletion != nil || doc.Editor.String() != before || doc.Dirty() {
					t.Fatalf("path should remain untouched, text=%q, popup open=%v", doc.Editor.String(), state.wordCompletion != nil)
				}
				return
			}
			if state.wordCompletion == nil {
				t.Fatalf("missing path popup: %q", state.notice)
			}
			if err := driver.Key(gfx.KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
			if got := doc.Editor.String(); got != `open "alpha.txt" tail` {
				t.Fatalf("completion did not replace the old suffix: %q", got)
			}
			state.command("Undo")
			if doc.Editor.String() != before || doc.Dirty() {
				t.Fatalf("Undo did not restore the saved buffer: %q", doc.Editor.String())
			}
			if data, err := os.ReadFile(file); err != nil || string(data) != before {
				t.Fatalf("completion changed source bytes: %q, %v", data, err)
			}
		})
	}
}
