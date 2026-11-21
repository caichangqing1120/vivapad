package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func TestNFOStyleChangesOnlyEditorPresentation(t *testing.T) {
	const line = "demo \u250c\u2500\u2510"
	original := []byte("\xef\xbb\xbf" + line + "\r\n")
	path := filepath.Join(t.TempDir(), "art.NFO")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	checkFace := func(want gfx.MonoFace, size float32) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		for _, painted := range driver.Snapshot().Texts {
			if painted.Value == line && painted.Style.MonoFace == want && painted.Style.Size == size {
				return
			}
		}
		t.Fatalf("text did not paint with face %v at size %v", want, size)
	}
	if !state.menuSelected("MS-DOS Style") {
		t.Fatal(".NFO did not automatically select MS-DOS Style")
	}
	checkFace(gfx.MonoClassic, 13)
	state.command("Zoom In")
	checkFace(gfx.MonoClassic, 14)
	state.command("Normal Text")
	checkFace(gfx.MonoSystem, 14)
	state.command("MS-DOS Style")
	checkFace(gfx.MonoClassic, 14)
	plain := state.space.NewDocument()
	plain.Editor.Insert("other pane")
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{doc, plain}
	state.paneOwner[doc], state.paneOwner[plain] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(plain))
	checkFace(gfx.MonoClassic, 14)
	plaintTextPainted := false
	for _, painted := range driver.Snapshot().Texts {
		if painted.Value == "other pane" {
			plaintTextPainted = true
			if painted.Style.MonoFace != gfx.MonoSystem {
				t.Fatal("the other pane inherited MS-DOS Style")
			}
		}
	}
	if !plaintTextPainted {
		t.Fatal("the other pane did not paint")
	}
	if doc.Encoding() != document.UTF8BOM || doc.Editor.String() != line+"\n" || doc.Dirty() {
		t.Fatalf("display mode changed document content or encoding: %q, %v", doc.Editor.String(), doc.Encoding())
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("disk bytes = %x, %v", got, err)
	}
}
