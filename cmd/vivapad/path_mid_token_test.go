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
		name, token, left, right string
		caret                    int
		noOp                     bool
	}{
		{"existing file", "alpha.txt", `"`, `"`, len("al"), true},
		{"unfinished final component", "al-old.txt", `"`, `"`, len("al"), false},
		{"exact filename prefix", "alpha.txt-old", `"`, `"`, len("alpha.txt"), false},
		{"interior directory component", "al/child.txt", `"`, `"`, len("al"), true},
		{"square bracket wrapper", "al-old.txt", "[", "]", len("al"), false},
		{"angle bracket wrapper", "al-old.txt", "<", ">", len("al"), false},
		{"relative path after another drive path", "al-old.txt", `C:\old then ./`, "", len("al"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "alpha.txt"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			prefix := "open " + tc.left
			before := prefix + tc.token + tc.right + " tail"
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
			doc.Editor.MoveTo(len(prefix)+tc.caret, false)
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
			if got, want := doc.Editor.String(), prefix+"alpha.txt"+tc.right+" tail"; got != want {
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

func TestPathCompletionPreservesFileCreatedWhilePopupIsOpen(t *testing.T) {
	root := t.TempDir()
	source, candidate, existing := filepath.Join(root, "buffer.txt"), filepath.Join(root, "alpha.txt"), filepath.Join(root, "al-old.txt")
	const before = `open [al-old.txt] tail`
	for name, value := range map[string]string{source: before, candidate: "suggested file"} {
		if err := os.WriteFile(name, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{source})
	doc := state.space.Current()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	clickID(t, driver, editorID)
	caret := len("open [al")
	doc.Editor.MoveTo(caret, false)
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, ' '); err != nil || !handled {
		t.Fatalf("path shortcut handled=%v err=%v", handled, err)
	}
	if popup := state.wordCompletion; popup == nil || len(popup.items) != 1 || popup.items[0] != "alpha.txt" {
		t.Fatalf("expected initial candidate for the unfinished path: %+v", popup)
	}
	if err := os.WriteFile(existing, []byte("created after the popup"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != before || doc.Dirty() || doc.Editor.Caret() != caret || doc.Editor.CanUndo() || state.wordCompletion != nil {
		t.Fatalf("completion replaced a newly existing path: text=%q dirty=%v caret=%d undo=%v popup=%+v", got, doc.Dirty(), doc.Editor.Caret(), doc.Editor.CanUndo(), state.wordCompletion)
	}
	for name, want := range map[string]string{source: before, candidate: "suggested file", existing: "created after the popup"} {
		if got, err := os.ReadFile(name); err != nil || string(got) != want {
			t.Fatalf("changed file %q: %q, %v", name, got, err)
		}
	}
}

func TestPathCompletionRefusesCandidateRemovedWhilePopupIsOpen(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "buffer.txt")
	candidate := filepath.Join(root, "alpha.txt")
	renamed := filepath.Join(root, "alpine.txt")
	const before = "open [al-old.txt] tail\r\n"
	for name, value := range map[string]string{source: before, candidate: "suggested file"} {
		if err := os.WriteFile(name, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{source})
	doc := state.space.Current()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	clickID(t, driver, editorID)
	caret := len("open [al")
	doc.Editor.MoveTo(caret, false)
	open := func(want string) {
		t.Helper()
		if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, ' '); err != nil || !handled {
			t.Fatalf("path shortcut handled=%v err=%v", handled, err)
		}
		if popup := state.wordCompletion; popup == nil || len(popup.items) != 1 || popup.items[0] != want {
			t.Fatalf("expected %q in the popup, got %+v", want, popup)
		}
	}
	open("alpha.txt")
	if err := os.Rename(candidate, renamed); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	const originalText = "open [al-old.txt] tail\n"
	if got := doc.Editor.String(); got != originalText || doc.Dirty() || doc.Editor.Caret() != caret || doc.Editor.CanUndo() || state.wordCompletion != nil {
		t.Fatalf("stale completion changed the editor: text=%q dirty=%v caret=%d undo=%v popup=%+v", got, doc.Dirty(), doc.Editor.Caret(), doc.Editor.CanUndo(), state.wordCompletion)
	}
	open("alpine.txt")
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != "open [alpine.txt] tail\n" || !doc.Dirty() {
		t.Fatalf("fresh completion did not replace the stale suffix: %q", got)
	}
	state.command("Undo")
	if doc.Editor.String() != originalText || doc.Dirty() {
		t.Fatalf("Undo did not restore the original saved text: %q", doc.Editor.String())
	}
	for name, want := range map[string]string{source: before, renamed: "suggested file"} {
		if got, err := os.ReadFile(name); err != nil || string(got) != want {
			t.Fatalf("completion changed %q: %q, %v", name, got, err)
		}
	}
}
