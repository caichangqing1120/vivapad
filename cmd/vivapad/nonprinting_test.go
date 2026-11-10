package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestSymbolFlagsRenderRepresentationsAndPreserveSourceOffsets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newModel()
	state.preferencesPath = filepath.Join(filepath.Dir(path), "settings.json")
	if state.showNonPrinting || !state.showControls || state.showWhitespace || state.showEOL || state.menuSelected("Show All Characters") {
		t.Fatal("incorrect source Show Symbol defaults")
	}
	doc := state.space.Current()
	const original = "A\u200bB\x01C\u2028D\nZ"
	doc.Editor.Insert(original)
	doc.Editor.ClearHistory()
	count := func() (int, string) {
		var canvas ui.Canvas
		state.paint(&canvas, ui.Size{Width: 640, Height: 400})
		var displayed strings.Builder
		for _, label := range canvas.Texts() {
			if label.RunCount > 0 {
				displayed.WriteString(label.Value)
			}
		}
		return len(canvas.Texts()), displayed.String()
	}
	_, shown := count()
	if !strings.Contains(shown, "SOH") || !strings.Contains(shown, "LS") || strings.Contains(shown, "ZWSP") {
		t.Fatalf("default represented text = %q", shown)
	}
	state.command("Show Non-Printing Characters")
	_, shown = count()
	if !strings.Contains(shown, "ZWSP") || doc.Editor.String() != original || doc.Editor.CanUndo() {
		t.Fatalf("nonprinting toggle changed buffer or failed to render: %q", shown)
	}
	state.command("Show All Characters")
	if !state.showWhitespace || !state.showEOL || !state.showNonPrinting || !state.showControls || !state.menuSelected("Show All Characters") {
		t.Fatal("Show All did not enable all four source flags")
	}
	state.command("Show All Characters")
	_, shown = count()
	if state.showWhitespace || state.showEOL || state.showNonPrinting || state.showControls || state.menuSelected("Show All Characters") ||
		strings.Contains(shown, "SOH") || strings.Contains(shown, "LS") || doc.Editor.String() != original || doc.Editor.CanUndo() {
		t.Fatalf("Show All off changed bytes or left visible representations: %q", shown)
	}
	state.command("Show Control Characters & Unicode EOL")
	state.command("Show Non-Printing Characters")
	state.command("Clone to Other View")
	if state.paneDocument(0) != doc || state.paneDocument(1) != doc {
		t.Fatal("source document was not cloned")
	}
	_, shown = count()
	if strings.Count(shown, "ZWSP") < 2 || strings.Count(shown, "SOH") < 2 || doc.Editor.String() != original {
		t.Fatalf("both panes did not paint representations: %q", shown)
	}
	restored := newStartupModel(nil, path)
	if !restored.showNonPrinting || !restored.showControls || restored.showWhitespace || restored.showEOL {
		t.Fatal("independent symbol flags did not survive restart")
	}
}

func TestSevenRowShowSymbolMenuFitsCompactWindow(t *testing.T) {
	state, driver := startDriver(t)
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
				if hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("Show Symbol row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("Show Symbol row %d missing", index)
		}
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact Show Symbol layout: %+v", problems)
	}
	clickID(t, driver, symbolScope.At(4))
	if !state.menuSelected("Show All Characters") || state.space.Current().Dirty() {
		t.Fatal("Show All menu did not activate or edited document")
	}
}

func TestCaptureNonPrintingMenuWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_NONPRINTING_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_NONPRINTING_SHOTS to capture symbol modes")
	}
	state := newModel()
	state.preferencesPath = filepath.Join(t.TempDir(), "VivaPad", "settings.json")
	state.space.Current().Editor.Insert("A\u200b B\x01 C\u2028 D\n")
	state.command("Show All Characters")
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
		if err := session.Screenshot(filepath.Join(output, "nonprinting-editor-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), menuScope.At(3))
		clickID(t, session.Driver(), menuItemScope.At(9))
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s symbol menu layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "nonprinting-menu-"+size.name+".png")); err != nil {
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
