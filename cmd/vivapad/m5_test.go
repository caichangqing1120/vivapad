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
	"vivapad/internal/preferences"
	"vivapad/internal/projects"
	"vivapad/internal/session"
)

func m5Driver(t *testing.T, paths ...string) (*model, *app.Driver) {
	t.Helper()
	state := newStartupModel(paths, filepath.Join(t.TempDir(), "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return state, driver
}

func TestM5StylePreviewCancelAndSave(t *testing.T) {
	state, driver := m5Driver(t)
	state.themeChoice = preferences.ModernDark
	clickID(t, driver, menuScope.At(6))
	clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[6], "Style Configurator...")))
	clickID(t, driver, styleSwatches.At(0))
	clickID(t, driver, styleSizeUpID)
	if state.palette().editorBackground != gfx.RGB24(0xffffff) || state.paneFontSize(0) != 14 {
		t.Fatalf("style draft did not preview: background=%+v size=%d row=%d colors=%q", state.palette().editorBackground, state.paneFontSize(0), state.m5.styleRow, state.m5.styleColors())
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.palette().editorBackground == gfx.RGB24(0xffffff) || state.paneFontSize(0) != 13 {
		t.Fatal("cancel retained draft")
	}
	state.command("Style Configurator...")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, styleSwatches.At(0))
	clickID(t, driver, styleApplyID)
	stored, err := preferences.Load(state.preferencesPath)
	if err != nil || stored.Styles.For(preferences.ModernDark)[0] != "#FFFFFF" || stored.Styles.For(preferences.ClassicLight)[0] != "" {
		t.Fatalf("saved style = %+v, %v", stored, err)
	}
	state.command(styleConfiguratorCommand)
	requireEditorCommandFocus(t, driver, m5Fields.At(0))
	if _, err := driver.ShortcutChord(primary(), 'a'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("#00zz00"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, styleApplyID)
	if state.m5 == nil {
		t.Fatal("invalid color was saved")
	}
	clickID(t, driver, styleResetID)
	clickID(t, driver, styleApplyID)
	stored, err = preferences.Load(state.preferencesPath)
	if err != nil || stored.Styles.For(preferences.ModernDark) != (preferences.StyleColors{}) || stored.FontSize != 13 {
		t.Fatalf("reset style = %+v, %v", stored, err)
	}
}

func TestM5ExternalReloadConfirmsDirtyAndComparePreservesBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := m5Driver(t, path)
	clickID(t, driver, editorID)
	if err := driver.Type("local "); err != nil {
		t.Fatal(err)
	}
	original := state.space.Current().Editor.String()
	replacement := filepath.Join(filepath.Dir(path), "replacement")
	if err := os.WriteFile(replacement, []byte("external\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	waitM5ExternalChange(t, state, driver)
	clickID(t, driver, externalCompareID)
	if state.comparison == nil || !state.comparison.saved || state.space.Current().Editor.String() != original {
		t.Fatal("compare must preview on-disk text without editing")
	}
	clickID(t, driver, externalReloadID)
	clickID(t, driver, reloadCancelID)
	if state.space.Current().Editor.String() != original {
		t.Fatal("cancel discarded local changes")
	}
	clickID(t, driver, externalReloadID)
	clickID(t, driver, reloadConfirmID)
	if state.space.Current().Editor.String() != "external\n" || len(state.externalChanges) != 0 {
		t.Fatal("confirmed reload did not consume external change")
	}
}

func waitM5ExternalChange(t *testing.T, state *model, driver *app.Driver) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); state.externalChanges[state.space.Current()] == ""; {
		if time.Now().After(deadline) {
			t.Fatal("external file change not observed")
		}
		time.Sleep(10 * time.Millisecond)
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestM5ExternalIgnoreAndWatchCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watched.txt")
	write := func(value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("original")
	state, driver := m5Driver(t, path)
	write("changed")
	waitM5ExternalChange(t, state, driver)
	clickID(t, driver, externalIgnoreID)
	if state.externalChanges[state.space.Current()] != "" || state.space.Current().Editor.String() != "original" {
		t.Fatal("Ignore altered the buffer or retained the prompt")
	}
	write("another change")
	waitM5ExternalChange(t, state, driver)
	watcher := state.externalWatcher
	state.command("Close")
	if state.externalWatcher != nil || len(state.externalBindings) != 0 {
		t.Fatal("closing the last watched tab leaked its directory watch")
	}
	select {
	case <-watcher.done:
	default:
		t.Fatal("watch worker is still alive")
	}
}

func TestM5ExternalChangeInInactiveTabCanBeIgnoredWithoutSwitching(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, driver := m5Driver(t, first, second)
	other, active := state.space.Documents[0], state.space.Current()
	if err := os.WriteFile(first, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(3 * time.Second); state.externalChanges[other] == ""; {
		if time.Now().After(deadline) {
			t.Fatal("inactive file change not observed")
		}
		time.Sleep(10 * time.Millisecond)
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
	clickID(t, driver, externalIgnoreID)
	if state.space.Current() != active || state.externalChanges[other] != "" {
		t.Fatal("Ignore switched tabs or left the inactive tab's prompt pending")
	}
}

func TestM5NewWindowUsesLiteralPath(t *testing.T) {
	state, driver := m5Driver(t)
	var args []string
	state.launchWindow = func(values ...string) error { args = slices.Clone(values); return nil }
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[0], "Open in New Window")))
	if !slices.Equal(args, []string{"--new-window", "--"}) {
		t.Fatalf("new empty window args = %q", args)
	}
	path := filepath.Join(t.TempDir(), "A & B.txt")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	state.openDocumentPaths([]string{path})
	state.command("Open in New Window")
	if len(args) != 3 || args[2] != state.space.Current().Path {
		t.Fatalf("new file window args = %q", args)
	}
}

func TestM5IndependentWindowLeavesMainSessionAndRecoveryAlone(t *testing.T) {
	root := t.TempDir()
	sessionPath := filepath.Join(root, "session.json")
	first := filepath.Join(root, "primary.txt")
	if err := session.Save(sessionPath, session.State{Active: first, Paths: []string{first}}); err != nil {
		t.Fatal(err)
	}
	recoveryPath := filepath.Join(root, "recovery.json")
	if err := os.WriteFile(recoveryPath, []byte("main recovery"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModelMode(nil, sessionPath, true)
	if state.space.Current().Path != "" || state.recoveryWriter != nil {
		t.Fatal("new blank window opened the primary session or shared recovery writer")
	}
	state.persistSession()
	stored, err := session.Load(sessionPath)
	if err != nil || stored.Active != first {
		t.Fatalf("main session was changed: %+v, %v", stored, err)
	}
	if content, err := os.ReadFile(recoveryPath); err != nil || string(content) != "main recovery" {
		t.Fatalf("main recovery was changed: %q, %v", content, err)
	}
}

func TestM5UDLMenuConfiguresExistingAndManuallySelectedFiles(t *testing.T) {
	root := t.TempDir()
	auto, manual := filepath.Join(root, "one.M5UDL"), filepath.Join(root, "two.txt")
	for _, path := range []string{auto, manual} {
		if err := os.WriteFile(path, []byte("let answer"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, driver := m5Driver(t, auto)
	state.themeChoice = preferences.ModernDark
	clickID(t, driver, menuScope.At(5))
	clickID(t, driver, udlManagerID)
	for index, value := range []string{"Demo Rules", "m5udl", "let pick", "##", "(*", "*)", `" '`} {
		clickID(t, driver, m5Fields.At(index))
		if err := driver.Type(value); err != nil {
			t.Fatal(err)
		}
	}
	clickID(t, driver, udlSaveID)
	if state.m5 != nil || state.languageLabel(state.space.Current()) != "Demo Rules" {
		t.Fatalf("UDL not applied to open extension match: %s", state.notice)
	}
	if _, err := driver.ShortcutChord(primary(), '/'); err != nil || !strings.HasPrefix(state.space.Current().Editor.String(), "##") {
		t.Fatalf("UDL toggle shortcut did not use configured line marker: %q, %v", state.space.Current().Editor.String(), err)
	}
	if !state.openDocumentPaths([]string{manual}) || state.languageLabel(state.space.Current()) == "Demo Rules" {
		t.Fatal("UDL was selected for an unrelated extension")
	}
	clickID(t, driver, menuScope.At(5))
	clickID(t, driver, menuItemScope.At(0))
	if state.languageLabel(state.space.Current()) != "Demo Rules" || state.udlHighlights[state.space.Current()].highlight == nil {
		t.Fatal("UDL menu did not enable highlighting")
	}
	state.command("Select All")
	state.command("Block Comment")
	if content := state.space.Current().Editor.String(); !strings.HasPrefix(content, "(*") || !strings.HasSuffix(content, "*)") {
		t.Fatalf("custom block comment = %q", content)
	}
	reopened := newStartupModel([]string{auto}, state.sessionPath)
	if reopened.languageLabel(reopened.space.Current()) != "Demo Rules" {
		t.Fatal("saved UDL was not loaded on startup")
	}
}

func TestM5ProjectPanelRenameOpenAndRemoveReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("keep this file"), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := m5Driver(t, path)
	state.command("Document List")
	state.command(projectPanelCommand)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, leftDockDocumentsID)
	clickID(t, driver, leftDockProjectsID)
	clickID(t, driver, projectNewID)
	if err := driver.Type("Work"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, projectSaveID)
	clickID(t, driver, projectAddID)
	clickID(t, driver, projectRenameID)
	if _, err := driver.ShortcutChord(primary(), 'a'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("Renamed"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, projectSaveID)
	groups, err := projects.Load(state.projectsPath)
	if err != nil || len(groups) != 1 || groups[0].Name != "Renamed" || len(groups[0].Files) != 1 {
		t.Fatalf("persisted projects = %+v, %v", groups, err)
	}
	state.command("New")
	clickID(t, driver, projectRows.At(1))
	if state.space.Current().Path != groups[0].Files[0] || len(state.space.Documents) != 2 {
		t.Fatal("project file did not focus existing tab")
	}
	clickID(t, driver, projectRemoveRows.At(0))
	clickID(t, driver, projectDeleteID)
	if groups, err := projects.Load(state.projectsPath); err != nil || len(groups) != 0 {
		t.Fatalf("removed project retained: %+v, %v", groups, err)
	}
	if content, err := os.ReadFile(path); err != nil || string(content) != "keep this file" {
		t.Fatalf("project operations changed disk file: %q, %v", content, err)
	}
}
