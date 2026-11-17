package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestPermissionRestoreOnWindowActivationReenablesFirstEditorKey(t *testing.T) {
	for _, pane := range []struct {
		name string
		id   gfx.ID
	}{
		{"main", editorID},
		{"split-right", rightEditorID},
	} {
		t.Run(pane.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notes.txt")
			if err := os.WriteFile(path, []byte("cat\n"), 0600); err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{path})
			doc := state.space.Current()
			if pane.id == rightEditorID {
				state.command("Clone to Other View")
			}
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			clickID(t, driver, pane.id)
			doc.Editor.MoveTo(0, false)
			if err := driver.Type("x"); err != nil {
				t.Fatal(err)
			}
			if doc.Editor.String() != "xcat\n" || !doc.Dirty() || doc.Editor.Caret() != 1 {
				t.Fatalf("dirty setup: text=%q dirty=%v caret=%d", doc.Editor.String(), doc.Dirty(), doc.Editor.Caret())
			}
			if err := os.Chmod(path, 0400); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0600) })
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			assertEditorHitReadOnly(t, driver, pane.id, true)
			state.command("Save")
			if data, err := os.ReadFile(path); err != nil || string(data) != "cat\n" || doc.Editor.String() != "xcat\n" || !doc.Dirty() || !doc.Editor.CanUndo() {
				t.Fatalf("failed Save changed disk or history: disk=%q err=%v text=%q dirty=%v", data, err, doc.Editor.String(), doc.Dirty())
			}
			if err := os.Chmod(path, 0600); err != nil {
				t.Fatal(err)
			}
			if err := driver.ActivateWindow(); err != nil {
				t.Fatal(err)
			}
			assertEditorHitReadOnly(t, driver, pane.id, false)
			if driver.Snapshot().Focused != pane.id {
				t.Fatalf("activation moved editor focus from %v to %v", pane.id, driver.Snapshot().Focused)
			}
			if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != "cat\n" {
				t.Fatalf("first Undo after permission restore: handled=%v err=%v text=%q", handled, err, doc.Editor.String())
			}
			if err := driver.Type("q"); err != nil || doc.Editor.String() != "qcat\n" || doc.Editor.Caret() != 1 {
				t.Fatalf("first typing after Undo: err=%v text=%q caret=%d", err, doc.Editor.String(), doc.Editor.Caret())
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "cat\n" {
				t.Fatalf("keyboard input changed disk without Save: %q, %v", data, err)
			}
		})
	}
}

func assertEditorHitReadOnly(t *testing.T, driver *app.Driver, id gfx.ID, want bool) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			if hit.ReadOnly != want {
				t.Fatalf("editor read-only hit = %v, want %v", hit.ReadOnly, want)
			}
			return
		}
	}
	t.Fatalf("editor hit %v missing", id)
}
