package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

func TestIndentGuideMenuPaintsBothViewsWithoutEditingAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	if !state.showIndentGuides {
		t.Fatal("indent guides should be enabled by default")
	}
	state.command("Show Indent Guide")
	doc := state.space.Current()
	doc.Editor.Insert("    parent\n\t\tchild\n  other")
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
	clickID(t, driver, symbolScope.At(5))
	if !state.showIndentGuides || !state.menuSelected("Show Indent Guide") || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("toggle edited text or did not enable guides: %q, notice=%q", doc.Editor.String(), state.notice)
	}
	guide := classicPalette.invisible
	guide.Alpha *= 0.5
	count := func() int {
		var canvas gfx.Canvas
		state.paint(&canvas, gfx.Size{Width: 640, Height: 400})
		marks := 0
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == guide {
				marks++
			}
		}
		return marks
	}
	one := count()
	if one == 0 {
		t.Fatal("visible leading indentation did not paint guides")
	}
	state.command("Clone to Other View")
	if two := count(); two <= one || state.paneDocument(0) != doc || state.paneDocument(1) != doc {
		t.Fatalf("split guides = %d, single=%d", two, one)
	}
	state.command("Show Indent Guide")
	if count() != 0 || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("disabling guides left paint or changed document")
	}
	state.command("Show Indent Guide")
	restored := newStartupModel(nil, path)
	if !restored.showIndentGuides {
		t.Fatal("indent guide preference did not survive restart")
	}
}

func TestCaptureIndentGuideMenuWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_INDENT_GUIDE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_INDENT_GUIDE_SHOTS to capture guide menu")
	}
	state := newStartupModel(nil, filepath.Join(t.TempDir(), "VivaPad", "session.json"))
	state.space.Current().Editor.Insert("func main() {\n    if ready {\n        run()\n    }\n}\n")
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
		if err := session.Screenshot(filepath.Join(output, "indent-guide-editor-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), menuScope.At(3))
		clickID(t, session.Driver(), menuItemScope.At(9))
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s guide layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "indent-guide-menu-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		if err := session.Driver().Key(gfx.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
		if err := session.Driver().Key(gfx.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
	}
}
