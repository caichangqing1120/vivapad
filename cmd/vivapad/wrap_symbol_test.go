package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestWrapSymbolMenuBothViewsWordWrapAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	if state.showWrapSymbol {
		t.Fatal("wrap symbol must start off")
	}
	doc := state.space.Current()
	doc.Editor.Insert(strings.Repeat("a long line with several words that wraps again ", 4))
	doc.Editor.ClearHistory()
	before := doc.Editor.String()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(9))
	for index := range symbolCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == symbolScope.At(index) {
				found = true
				if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("symbol row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("symbol row %d missing", index)
		}
	}
	clickID(t, driver, symbolScope.At(6))
	if !state.showWrapSymbol || !state.menuSelected("Show Wrap Symbol") || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("wrap symbol toggle edited text or failed to check menu")
	}
	count := func() int {
		var canvas ui.Canvas
		state.paint(&canvas, ui.Size{Width: 640, Height: 400})
		marks := 0
		for _, label := range canvas.Texts() {
			if label.Value == "↪" {
				marks++
			}
		}
		return marks
	}
	if count() != 0 {
		t.Fatal("wrap symbols appeared while Word Wrap was off")
	}
	state.command("Word Wrap")
	one := count()
	if one == 0 {
		t.Fatal("Word Wrap did not show soft-wrap end markers")
	}
	caret, visible := state.codeEditor().CaretBounds()
	if !visible {
		t.Fatal("wrapped caret is not visible")
	}
	if offset, ok := state.codeEditor().OffsetAt(ui.Point{X: caret.X + 0.1, Y: caret.Y + caret.Height/2}); !ok || offset != doc.Editor.ByteOffset(doc.Editor.Caret()) {
		t.Fatalf("wrapped caret hit offset = %d, %v; want %d", offset, ok, doc.Editor.ByteOffset(doc.Editor.Caret()))
	}
	state.command("Clone to Other View")
	if two := count(); two <= one || state.paneDocument(0) != doc || state.paneDocument(1) != doc {
		t.Fatalf("split markers=%d, single=%d", two, one)
	}
	state.command("Word Wrap")
	if count() != 0 || !state.menuSelected("Show Wrap Symbol") {
		t.Fatal("disabling Word Wrap cleared the preference or kept markers")
	}
	state.command("Word Wrap")
	if count() == 0 || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("re-enabling Word Wrap lost markers or changed text history")
	}
	restored := newStartupModel(nil, path)
	if !restored.showWrapSymbol {
		t.Fatal("wrap symbol preference did not survive restart")
	}
}

func TestCaptureWrapSymbolWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_WRAP_SYMBOL_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_WRAP_SYMBOL_SHOTS to capture wrap markers")
	}
	state := newStartupModel(nil, filepath.Join(t.TempDir(), "VivaPad", "session.json"))
	state.space.Current().Editor.Insert(strings.Repeat("the first line has enough words to wrap in every viewport, ", 4) + "\nshort\n")
	state.command("Word Wrap")
	state.command("Show Wrap Symbol")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 800}, {"compact", 640, 400},
	} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if err := session.Screenshot(filepath.Join(output, "wrap-symbol-editor-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), menuScope.At(3))
		clickID(t, session.Driver(), menuItemScope.At(9))
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s wrap symbol layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "wrap-symbol-menu-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		if err := session.Driver().Key(ui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
		if err := session.Driver().Key(ui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
	}
}
