package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"vivapad/internal/preferences"
)

func pressPaletteShortcut(t *testing.T, driver *app.Driver, modifiers gfx.Modifiers) {
	t.Helper()
	if handled, err := driver.ShortcutChord(primary()|modifiers, 'p'); !handled || err != nil {
		t.Fatalf("palette shortcut handled=%t: %v", handled, err)
	}
}

func TestCommandPaletteFuzzyKeyboardExecutionAndEscape(t *testing.T) {
	state, driver := startDriver(t)
	state.chineseUI = false
	clickID(t, driver, editorID)
	pressPaletteShortcut(t, driver, gfx.ModifierShift)
	if driver.Snapshot().Focused != paletteInputID {
		t.Fatal("command palette did not focus its search input")
	}
	if err := driver.Type("fmtjs"); err != nil {
		t.Fatal(err)
	}
	if len(state.commandPalette.matches) != 2 || !strings.Contains(state.commandPalette.matches[0].label, "JSON") {
		t.Fatalf("unexpected fuzzy matches: %+v", state.commandPalette.matches)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.commandPalette != nil || driver.Snapshot().Focused != editorID {
		t.Fatalf("Escape did not close the palette and restore editor focus: %v", err)
	}
	pressPaletteShortcut(t, driver, gfx.ModifierShift)
	if err := driver.Type("switch theme"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []gfx.Key{gfx.KeyDown, gfx.KeyDown, gfx.KeyUp, gfx.KeyEnter} {
		if err := driver.Key(key, 0); err != nil {
			t.Fatal(err)
		}
	}
	if state.commandPalette != nil || state.themeChoice != preferences.ModernDark {
		t.Fatalf("keyboard execution selected the wrong theme: %v", state.themeChoice)
	}
}

func TestQuickSwitcherPaletteSwitchesTabsAndFindsCollapsedWorkspaceFiles(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Name = "First Log.txt"
	state.showInActivePane(state.space.NewDocument())
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(nested, "workspace.json")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	pressPaletteShortcut(t, driver, 0)
	if err := driver.Type("fltxt"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil || state.space.Current() != first || state.commandPalette != nil {
		t.Fatalf("quick switcher did not select the open tab: %v", err)
	}
	pressPaletteShortcut(t, driver, 0)
	if err := driver.Type(`nested\wj`); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for len(state.commandPalette.matches) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil || state.space.Current().Name != "workspace.json" {
		t.Fatalf("quick switcher did not open the nested workspace file: %v", err)
	}
}

func TestCommandPaletteBilingualLabels(t *testing.T) {
	for _, locale := range []struct {
		chinese bool
		query   string
		label   string
	}{{false, "偏好设置", "Preferences"}, {true, "preferences", "偏好设置"}} {
		state, driver := startDriver(t)
		state.chineseUI = locale.chinese
		pressPaletteShortcut(t, driver, gfx.ModifierShift)
		if err := driver.Type(locale.query); err != nil {
			t.Fatal(err)
		}
		matches := state.commandPalette.matches
		if len(matches) != 1 || matches[0].label != locale.label {
			t.Fatalf("query %q: got %+v, want %q", locale.query, matches, locale.label)
		}
		clickID(t, driver, paletteRowScope.At(0))
		if !state.preferencesOpen || state.commandPalette != nil {
			t.Fatal("clicking Preferences did not transfer to the dialog")
		}
	}
}

func TestToolbarQuickThemeAndSettingsDoNotOverlap(t *testing.T) {
	state, driver := startDriver(t)
	for _, width := range []int{1280, 800, 640, 320} {
		if err := driver.Resize(width, 480); err != nil {
			t.Fatal(err)
		}
		hits := driver.Snapshot().Hits
		for _, id := range []gfx.ID{toolbarThemeID, toolbarSettingsID} {
			found := false
			for _, hit := range hits {
				if hit.ID != id {
					continue
				}
				found = true
				if hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > float32(width) {
					t.Fatalf("quick action clipped at %d: %+v", width, hit.Bounds)
				}
				if err := driver.Move(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
					t.Fatal(err)
				}
				if !slices.ContainsFunc(driver.Snapshot().Texts, func(label gfx.Text) bool {
					return label.Layer > 0 && label.Value == hit.Label
				}) {
					t.Fatalf("missing tooltip for %q at %d", hit.Label, width)
				}
				for _, other := range hits {
					if other.ID != id && other.Bounds.Y >= menuHeight && other.Bounds.Y < menuHeight+toolbarHeight &&
						hit.Bounds.X < other.Bounds.X+other.Bounds.Width && other.Bounds.X < hit.Bounds.X+hit.Bounds.Width {
						t.Fatalf("quick action %q overlaps %q at %d", hit.Label, other.Label, width)
					}
				}
			}
			if !found {
				t.Fatalf("missing quick action at %d", width)
			}
		}
		if width >= 640 {
			if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
				t.Fatalf("layout at %d: %+v", width, problems)
			}
		}
	}
	state.themeChoice = preferences.ClassicLight
	clickID(t, driver, toolbarThemeID)
	if state.themeChoice != preferences.ModernDark {
		t.Fatalf("theme quick toggle = %v", state.themeChoice)
	}
	clickID(t, driver, toolbarThemeID)
	if state.themeChoice != preferences.ClassicLight {
		t.Fatalf("theme quick toggle did not return to light: %v", state.themeChoice)
	}
	clickID(t, driver, toolbarSettingsID)
	if !state.preferencesOpen {
		t.Fatal("settings quick action did not open Preferences")
	}
}

func TestSaveToastExpiresWithoutClearingStatus(t *testing.T) {
	state, driver := startDriver(t)
	state.chineseUI = false
	path := filepath.Join(t.TempDir(), "saved.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveID)
	notice := state.notice
	if notice != "Saved saved.txt" || state.toast.message != notice || !state.toast.until.After(time.Now()) {
		t.Fatalf("save did not show its confirmation: %q / %+v", notice, state.toast)
	}
	isToastLabel := func(label gfx.Text) bool { return label.Value == notice && label.Layer > 0 }
	if !slices.ContainsFunc(driver.Snapshot().Texts, isToastLabel) {
		t.Fatal("save confirmation never painted on the overlay")
	}
	state.toast.until = time.Now().Add(-time.Second)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.toast != (toastState{}) || state.notice != notice {
		t.Fatalf("expiring toast changed persistent status: %q / %+v", state.notice, state.toast)
	}
	if slices.ContainsFunc(driver.Snapshot().Texts, isToastLabel) {
		t.Fatal("expired toast is still visible")
	}
}
