package main

import (
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func TestViewSymbolMenuFitsCompactAndDoesNotEdit(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("a \tb\nc")
	doc.Editor.ClearHistory()
	before := doc.Editor.String()
	state.command("Read-Only on Current Document")
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(9))
	for index := range symbolCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == symbolScope.At(index) {
				found = true
				if hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("symbol menu row clipped: %+v", hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("symbol menu item %d missing", index)
		}
	}
	clickID(t, driver, symbolScope.At(0))
	if !state.showWhitespace || doc.Editor.String() != before || doc.Editor.CanUndo() || !state.isReadOnly(doc) {
		t.Fatalf("space/tab toggle changed document: %q", doc.Editor.String())
	}
	var canvas gfx.Canvas
	state.paint(&canvas, gfx.Size{Width: 640, Height: 400})
	marks := 0
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == symbolColor {
			marks++
		}
	}
	if marks < 2 {
		t.Fatalf("space/tab marks missing: %d", marks)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(9))
	clickID(t, driver, symbolScope.At(1))
	if !state.showEOL || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("EOL visibility edited the buffer")
	}
	seenLF := false
	for _, label := range driver.Snapshot().Texts {
		seenLF = seenLF || label.Value == "LF"
	}
	if !seenLF {
		t.Fatal("LF label was not painted")
	}
	doc.SetLineEnding(document.CRLF)
	canvas.Reset()
	state.paint(&canvas, gfx.Size{Width: 640, Height: 400})
	seenCRLF := false
	for _, label := range canvas.Texts() {
		seenCRLF = seenCRLF || label.Value == "CRLF"
	}
	if !seenCRLF {
		t.Fatal("CRLF label did not follow document disk format")
	}
}

func TestSymbolSettingsPersistAndApplyToBothSplitViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	state.command("Show Space and Tab")
	state.command("Show End of Line")
	restored := newStartupModel(nil, path)
	if !restored.showWhitespace || !restored.showEOL {
		t.Fatalf("symbol preferences lost on restart: whitespace=%v eol=%v", restored.showWhitespace, restored.showEOL)
	}
	restored.space.Current().Editor.Insert("a\nb")
	driver, err := app.NewDriver(newConfig(restored), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	count := 0
	for _, label := range driver.Snapshot().Texts {
		if label.Value == "LF" {
			count++
		}
	}
	if count != 2 || restored.space.Current().Editor.String() != "a\nb" {
		t.Fatalf("two-view EOL labels=%d text=%q", count, restored.space.Current().Editor.String())
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
}
