package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"vivapad/internal/preferences"
)

func TestPreferencesDialogEditsExistingSettingsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	state := newStartupModel(nil, path)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.space.Current().Editor.Insert("unchanged")
	state.space.Current().Editor.ClearHistory()
	state.command("Split View")
	clickID(t, driver, menuScope.At(6))
	clickID(t, driver, menuItemScope.At(0))
	if !state.preferencesOpen {
		t.Fatal("Settings first command did not open Preferences")
	}
	clickID(t, driver, preferencesTabScope.At(2))
	clickID(t, driver, preferencesIndentScope.At(2))
	clickID(t, driver, preferencesAutoIndentID)
	clickID(t, driver, preferencesPairBracketsID)
	clickID(t, driver, preferencesViewID)
	clickID(t, driver, preferencesLineNumbersID)
	clickID(t, driver, preferencesWrapID)
	clickID(t, driver, preferencesWhitespaceID)
	clickID(t, driver, preferencesEOLID)
	clickID(t, driver, preferencesZoomSyncID)
	if state.tabWidth != 8 || state.indentMode != "spaces" || state.autoIndent || state.autoClose || state.showLineNumbers || !state.wrap || !state.showWhitespace || !state.showEOL || !state.zoomSync {
		t.Fatal("Preferences did not apply existing editor options")
	}
	clickID(t, driver, newID)
	if _, err := driver.ShortcutChord(primary(), 'n'); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 1 || state.space.Current().Editor.String() != "unchanged" || state.space.Current().Editor.CanUndo() {
		t.Fatal("Preferences allowed editing behind the modal")
	}
	dropped := filepath.Join(t.TempDir(), "dropped.txt")
	if err := os.WriteFile(dropped, []byte("drop"), 0600); err != nil {
		t.Fatal(err)
	}
	newConfig(state).OnFileDrop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, Paths: []string{dropped}})
	if len(state.space.Documents) != 1 {
		t.Fatal("Preferences allowed a file drop behind the modal")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.preferencesOpen {
		t.Fatalf("Escape failed to close Preferences: %v", err)
	}
	stored, err := preferences.Load(filepath.Join(filepath.Dir(path), "settings.json"))
	if err != nil || stored.TabWidth != 8 || stored.IndentMode != "spaces" || stored.AutoIndent || stored.AutoClose || stored.LineNumbers || !stored.Wrap || !stored.ShowWhitespace || !stored.ShowEOL || !stored.ZoomSync {
		t.Fatalf("stored Preferences = %+v, %v", stored, err)
	}
	restarted := newStartupModel(nil, path)
	if restarted.tabWidth != 8 || restarted.indentMode != "spaces" || !restarted.zoomSync || !restarted.showEOL {
		t.Fatal("Preferences did not survive restart")
	}
}

func TestPreferencesDialogReportsPrivateStorageFailure(t *testing.T) {
	state, driver := startDriver(t)
	state.preferencesPath = filepath.Join(t.TempDir(), "settings.json")
	if err := os.Mkdir(state.preferencesPath, 0700); err != nil {
		t.Fatal(err)
	}
	state.command("Preferences...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, preferencesAutoIndentID)
	if !strings.Contains(state.notice, "Settings not saved") {
		t.Fatalf("failed preference save notice = %q", state.notice)
	}
	state.preferencesPath, state.notice = "", ""
	clickID(t, driver, preferencesAutoIndentID)
	if !strings.Contains(state.notice, "Settings not saved") {
		t.Fatalf("missing preference storage notice = %q", state.notice)
	}
}

func TestPreferencesDialogCompactModalAndCloseGuard(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.command("Preferences...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []gfx.ID{preferencesEditingID, preferencesViewID, preferencesTabScope.At(0), preferencesCloseID} {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == id {
				found = true
				if hit.Bounds.X < 0 || hit.Bounds.Y < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("Preferences control outside compact window: %+v", hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("Preferences control %v missing", id)
		}
	}
	closed := false
	newConfig(state).OnCloseRequest(func() { closed = true })
	if closed || !state.preferencesOpen {
		t.Fatal("window close bypassed Preferences")
	}
	clickID(t, driver, preferencesCloseID)
	if state.preferencesOpen {
		t.Fatal("Preferences Close button did not dismiss the dialog")
	}
}

func TestPreferencesDialogCompactChoicesAndLocalizedNavigation(t *testing.T) {
	state, driver := startDriver(t)
	state.chineseUI, state.uiTextSize = true, "large"
	state.command("Preferences...")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	wants := map[gfx.ID]struct {
		label string
		role  gfx.Role
	}{
		preferencesGeneralID:      {"通用", gfx.RoleTab},
		preferencesTabScope.At(0): {"2", gfx.RoleRadio},
		preferencesAutoIndentID:   {"自动缩进", gfx.RoleCheckbox},
	}
	for _, hit := range driver.Snapshot().Hits {
		if want, ok := wants[hit.ID]; ok {
			if hit.Label != want.label || hit.Role != want.role {
				t.Errorf("hit %v = %q/%v, want %q/%v", hit.ID, hit.Label, hit.Role, want.label, want.role)
			}
			if hit.Role == gfx.RoleRadio && hit.Bounds.Width > 60 {
				t.Errorf("numeric choice should not stretch across the row: %+v", hit.Bounds)
			}
			delete(wants, hit.ID)
		}
	}
	if len(wants) > 0 {
		t.Fatalf("missing hits: %v", wants)
	}
}

func TestPreferencesDialogLayoutAcrossThemesAndLanguages(t *testing.T) {
	for _, theme := range []preferences.Theme{preferences.ClassicLight, preferences.ModernDark, preferences.Monokai} {
		for _, language := range []string{"en", "zh-CN"} {
			t.Run(fmt.Sprintf("%s/%s", theme, language), func(t *testing.T) {
				state, driver := startDriver(t)
				state.themeChoice, state.language, state.uiTextSize = theme, language, "large"
				state.resolveInterfaceLanguage()
				state.command("Preferences...")
				for _, size := range [][2]int{{640, 400}, {1280, 800}} {
					if err := driver.Resize(size[0], size[1]); err != nil {
						t.Fatal(err)
					}
					for _, id := range []gfx.ID{preferencesGeneralID, preferencesEditingID, preferencesViewID, preferencesSymbolsID, preferencesPrintID, preferencesPrintBandsID, preferencesPrintFontsID, preferencesAboutID} {
						clickID(t, driver, id)
						checkPreferencesLayout(t, driver, state.preferencesPage)
						if id == preferencesPrintBandsID {
							clickID(t, driver, printVariableButtonID)
							checkPreferencesLayout(t, driver, state.preferencesPage)
							clickID(t, driver, printVariableScope.At(0))
						}
					}
				}
			})
		}
	}
}

func checkPreferencesLayout(t *testing.T, driver *app.Driver, pageIndex int) {
	t.Helper()
	snapshot := driver.Snapshot()
	if problems := headless.CheckLayout(snapshot); len(problems) != 0 {
		t.Errorf("page %d at %dx%d: %+v", pageIndex, snapshot.Width, snapshot.Height, problems)
	}
	for _, text := range snapshot.Texts {
		if text.Layer < 4 || !strings.HasSuffix(text.Value, "…") {
			continue
		}
		if pageIndex == preferencesAboutPage && (strings.HasPrefix(text.Value, "Build Commit:") || strings.HasPrefix(text.Value, "构建提交：") || strings.HasPrefix(text.Value, "Viva:")) {
			continue
		}
		t.Errorf("page %d at %dx%d: truncated label %q", pageIndex, snapshot.Width, snapshot.Height, text.Value)
	}
}

func TestCapturePreferencesDialogWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_PREFERENCES_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_PREFERENCES_SHOTS to capture Preferences")
	}
	state := newModel()
	state.command("Preferences...")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if size.name == "compact" {
			clickID(t, session.Driver(), preferencesViewID)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s Preferences layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "preferences-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	state.preferencesOpen = false
	state.invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), menuScope.At(6))
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("Settings menu layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "preferences-menu-compact.png")); err != nil {
		t.Fatal(err)
	}
}
