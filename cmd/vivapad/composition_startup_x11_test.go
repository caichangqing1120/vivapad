//go:build linux && !vivagtk

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
)

func TestNativeX11RequestsTentativeCompositionWithLegacyFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("cat\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	config := newNativeConfig(state)
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	input := config.TextInput(app.TextInputContext{ID: editorID, Multiline: true})
	if input == nil || !input.State.TentativeComposition || !doc.Editor.TentativeCompositionEnabled() {
		t.Fatal("direct X11 startup did not request tentative input for the document")
	}
	if state.query.TentativeCompositionEnabled() {
		t.Fatal("document startup request changed the search field")
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, Text: "candidate", CompositionPhase: text.CompositionUpdate}); err != nil {
		t.Fatal(err)
	}
	if !doc.Editor.TentativeCompositionActive() || doc.Editor.CommittedString() != "cat\n" {
		t.Fatal("tagged update was not tentative")
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextCancelComposition, CompositionPhase: text.CompositionCancel}); err != nil {
		t.Fatal(err)
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, Text: "legacy"}); err != nil {
		t.Fatal(err)
	}
	if !doc.Editor.CompositionActive() || doc.Editor.TentativeCompositionActive() || doc.Editor.String() != "cat\nlegacy" {
		t.Fatal("untagged update did not use legacy input")
	}
	if !state.save(state.space.Active, false) {
		t.Fatalf("save failed: %s", state.notice)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "cat\n" || doc.Editor.CompositionActive() || doc.Editor.CanUndo() {
		t.Fatalf("legacy fallback saved provisional text or left history: data=%q err=%v", data, err)
	}
}
