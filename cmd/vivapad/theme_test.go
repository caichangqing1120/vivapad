package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/preferences"
	"vivapad/internal/syntax"
)

func TestThemeViaPreferencesAndViewMenuKeepsSplitDocumentsIntact(t *testing.T) {
	dir := t.TempDir()
	session := filepath.Join(dir, "session.json")
	path := filepath.Join(dir, "example.go")
	const source = "package demo\n\n// Greeting\nfunc greet() string { return \"hi\" }\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, session)
	state.command("Clone to Other View")
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	assertThemePanes(t, driver, gfx.RGB24(0xffffff), 2)
	assertCommentColorInPanes(t, driver, gfx.RGB24(0x5c7d57))
	clickID(t, driver, menuScope.At(6))
	clickID(t, driver, menuItemScope.At(0))
	clickID(t, driver, preferencesViewID)
	clickID(t, driver, preferencesThemeScope.At(1))
	assertThemePanes(t, driver, gfx.RGB24(0x1e1f22), 2)
	assertCommentColorInPanes(t, driver, gfx.RGB24(0xaab5bf))
	if driver.NativeAppearance() != app.AppearanceDark {
		t.Fatal("Modern Dark did not request dark window chrome")
	}
	clickID(t, driver, preferencesCloseID)
	clickID(t, driver, menuScope.At(3))
	revealViewMenuItem(t, driver, len(menuCommands[3])-1)
	clickID(t, driver, menuItemScope.At(len(menuCommands[3])-1))
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.themeMenuOpen || state.menuOpen != 3 {
		t.Fatalf("Escape should close only the submenu: %v", err)
	}
	clickID(t, driver, menuItemScope.At(len(menuCommands[3])-1))
	clickID(t, driver, themeMenuScope.At(2))
	assertThemePanes(t, driver, gfx.RGB24(0x272822), 2)
	assertCommentColorInPanes(t, driver, gfx.RGB24(0xb5ae94))
	for pane := range 2 {
		doc := state.paneDocument(pane)
		if doc.Dirty() || doc.Editor.String() != source || doc.Editor.CanUndo() || doc.Editor.CanRedo() {
			t.Fatalf("pane %d changed its document or undo state", pane)
		}
	}
	reopened := newStartupModel([]string{path}, session)
	if reopened.themeChoice != preferences.Monokai {
		t.Fatalf("reopened theme = %q", reopened.themeChoice)
	}
	driver.Close()
	reopenedDriver, err := app.NewDriver(newConfig(reopened), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reopenedDriver.Close()
	assertThemePanes(t, reopenedDriver, gfx.RGB24(0x272822), 2)
}

func TestThemeFollowsLiveDriverAppearanceOnlyWhenSelected(t *testing.T) {
	state := newStartupModel(nil, filepath.Join(t.TempDir(), "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Theme: Follow System")
	pump := func() {
		t.Helper()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
	pump()
	assertThemePanes(t, driver, gfx.RGB24(0xffffff), 1)
	if driver.NativeAppearance() != app.AppearanceUnknown {
		t.Fatal("Follow System should not pin window chrome")
	}
	for _, appearance := range []app.Appearance{app.AppearanceDark, app.AppearanceLight, app.AppearanceDark, app.AppearanceUnknown} {
		driver.SetSystemAppearance(appearance)
		pump()
		want := gfx.RGB24(0xffffff)
		if appearance == app.AppearanceDark {
			want = gfx.RGB24(0x1e1f22)
		}
		assertThemePanes(t, driver, want, 1)
	}
	stored, err := preferences.Load(state.preferencesPath)
	if err != nil || stored.Theme != preferences.FollowSystem {
		t.Fatalf("system changes overwrote the saved choice: %q, %v", stored.Theme, err)
	}
	state.command("Theme: Classic Light")
	driver.SetSystemAppearance(app.AppearanceDark)
	pump()
	assertThemePanes(t, driver, gfx.RGB24(0xffffff), 1)
}

func TestThemePaintsAllMarkStylesAndSelectionInDarkEditors(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const value = "alpha bravo charlie delta echo find select"
	doc.Editor.Insert(value)
	for index, token := range []string{"alpha", "bravo", "charlie", "delta", "echo"} {
		doc.Editor.MoveTo(strings.Index(value, token)+1, false)
		state.styleToken(index, false)
	}
	setMarkQuery(state, "find")
	state.markCurrentDocument()
	doc.Editor.SetSelection(text.Range{Start: strings.Index(value, "select"), End: len(value)})
	for _, option := range themeOptions[1:3] {
		t.Run(option.label, func(t *testing.T) {
			state.command(option.command())
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			palette := state.palette()
			frame := driver.Snapshot()
			colors := append([]gfx.Color{palette.findMark, palette.selection}, palette.tokenMarks[:]...)
			found := make(map[gfx.Color]bool)
			for _, primitive := range frame.Primitives {
				found[primitive.Color] = true
				if primitive.Color == gfx.RGB24(0xffffff) {
					t.Fatalf("dark editor emitted a white rectangle at %v", primitive.Bounds)
				}
			}
			for _, color := range colors {
				if !found[color] {
					t.Errorf("missing mark or selection fill %v", color)
				}
			}
			if contrastRatio(palette.selectionText, palette.selection) < 4.5 {
				t.Error("selected text needs at least 4.5:1 contrast")
			}
			var highlighter syntax.Highlighter
			runs := state.renderTheme.Recolor(highlighter.Highlight(".go", `return "hello" // note`, nil))
			backgrounds := append([]gfx.Color{palette.editorBackground, palette.findMark, palette.findMatch}, palette.tokenMarks[:]...)
			for _, background := range backgrounds {
				if contrastRatio(palette.editorForeground, background) < 4.5 {
					t.Errorf("editor text has low contrast on %v", background)
				}
				for _, run := range runs {
					if contrastRatio(run.Color, background) < 4.5 {
						t.Errorf("token color %v has low contrast on %v", run.Color, background)
					}
				}
			}
		})
	}
	if doc.Editor.String() != value || !doc.Editor.Undo() || doc.Editor.String() != "" {
		t.Fatal("theme changes did not preserve the original undo entry")
	}
}

func contrastRatio(first, second gfx.Color) float64 {
	luminance := func(color gfx.Color) float64 {
		linear := func(channel float32) float64 {
			value := float64(channel)
			if value <= 0.04045 {
				return value / 12.92
			}
			return math.Pow((value+0.055)/1.055, 2.4)
		}
		return 0.2126*linear(color.Red) + 0.7152*linear(color.Green) + 0.0722*linear(color.Blue)
	}
	firstLuma, secondLuma := luminance(first), luminance(second)
	return (max(firstLuma, secondLuma) + 0.05) / (min(firstLuma, secondLuma) + 0.05)
}

func assertThemePanes(t *testing.T, driver *app.Driver, background gfx.Color, wantCount int) {
	t.Helper()
	frame := driver.Snapshot()
	count := 0
	for _, hit := range frame.Hits {
		if hit.ID != editorID && hit.ID != rightEditorID {
			continue
		}
		count++
		found := false
		for _, primitive := range frame.Primitives {
			if primitive.Color == background && primitive.Bounds.X >= hit.Bounds.X && primitive.Bounds.Y >= hit.Bounds.Y && primitive.Bounds.Width >= hit.Bounds.Width-6 && primitive.Bounds.Height >= hit.Bounds.Height-6 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("pane %v has no %v background", hit.ID, background)
		}
	}
	if count != wantCount {
		t.Fatalf("editor pane count = %d, want %d", count, wantCount)
	}
}

func assertCommentColorInPanes(t *testing.T, driver *app.Driver, color gfx.Color) {
	t.Helper()
	frame := driver.Snapshot()
	count := 0
	for _, label := range frame.Texts {
		if label.Value != "// Greeting" {
			continue
		}
		for _, run := range frame.TextRuns(label) {
			if run.Color == color && int(run.Bytes) == len(label.Value) {
				count++
			}
		}
	}
	if count != 2 {
		t.Fatalf("%d panes drew the comment in %v; want 2", count, color)
	}
}
