package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/macro"
	"vivapad/internal/preferences"
	"vivapad/internal/runcommand"
)

func TestShortcutMapperRebindsAndRestoresFocus(t *testing.T) {
	state := newStartupModel(nil, filepath.Join(t.TempDir(), "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	clickID(t, driver, editorID)
	clickID(t, driver, menuScope.At(6))
	clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[6], shortcutMapperCommand)))
	requireEditorCommandFocus(t, driver, mapperSearchID)
	if err := driver.Type("New"); err != nil {
		t.Fatal(err)
	}
	clickMapperRow(t, driver, "New")
	clickID(t, driver, mapperCaptureID)
	if _, err := driver.ShortcutChord(primary(), 's'); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(state.mapper.message, "Save") {
		t.Fatalf("missing conflict: %q", state.mapper.message)
	}
	clickID(t, driver, mapperCaptureID)
	if err := driver.Key(gfx.KeyF9, 0); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, mapperApplyID)
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	requireEditorCommandFocus(t, driver, editorID)
	if _, err := driver.ShortcutChord(primary(), 'n'); err != nil || len(state.space.Documents) != 1 {
		t.Fatalf("old shortcut still active: %v", err)
	}
	if err := driver.Key(gfx.KeyF9, 0); err != nil || len(state.space.Documents) != 2 {
		t.Fatalf("replacement did not create a document: %v", err)
	}
	opened := newStartupModel(nil, state.sessionPath)
	if got := opened.shortcuts.Binding("New").Key; got != "F9" {
		t.Fatalf("reopened shortcut = %q", got)
	}
}

func TestShortcutMapperSearchesBindingAndRunsSavedCommand(t *testing.T) {
	state, driver := startDriver(t)
	state.themeChoice = preferences.Monokai
	state.namedRuns = []runcommand.Named{{Name: "Inspect", Template: "inspect"}}
	launched := ""
	state.runLauncher = func(prepared runcommand.Prepared) error { launched = prepared.Executable; return nil }
	state.command(shortcutMapperCommand)
	requireEditorCommandFocus(t, driver, mapperSearchID)
	if err := driver.Type("F8"); err != nil {
		t.Fatal(err)
	}
	clickMapperRow(t, driver, "Next Difference")
	clickID(t, driver, mapperSearchID)
	if _, err := driver.ShortcutChord(primary(), 'a'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("Inspect"); err != nil {
		t.Fatal(err)
	}
	clickMapperRow(t, driver, "Run: Inspect")
	clickID(t, driver, mapperCaptureID)
	if err := driver.Key(gfx.KeyF9, 0); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, mapperApplyID)
	clickID(t, driver, mapperCloseID)
	requireEditorCommandFocus(t, driver, editorID)
	if err := driver.Key(gfx.KeyF9, 0); err != nil || launched != "inspect" {
		t.Fatalf("saved shortcut launched %q: %v", launched, err)
	}
}

func TestShortcutMapperClearResetAndCancelInSplit(t *testing.T) {
	state, driver := startDriver(t)
	state.themeChoice = preferences.ModernDark
	state.command("Split View")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	open := func() {
		state.command(shortcutMapperCommand)
		requireEditorCommandFocus(t, driver, mapperSearchID)
		if err := driver.Type("New"); err != nil {
			t.Fatal(err)
		}
		clickMapperRow(t, driver, "New")
	}
	open()
	clickID(t, driver, mapperClearID)
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	requireEditorCommandFocus(t, driver, rightEditorID)
	if state.shortcuts.Binding("New").Empty() {
		t.Fatal("Escape saved a pending clear")
	}
	open()
	clickID(t, driver, mapperClearID)
	clickID(t, driver, mapperApplyID)
	if !state.shortcuts.Binding("New").Empty() {
		t.Fatal("Apply did not clear the binding")
	}
	clickID(t, driver, mapperResetID)
	if state.shortcuts.Binding("New").Key != "N" {
		t.Fatal("Reset Binding did not restore New")
	}
	clickID(t, driver, mapperClearID)
	clickID(t, driver, mapperApplyID)
	clickID(t, driver, mapperResetAll)
	if state.shortcuts.Binding("New").Key != "N" {
		t.Fatal("Reset All did not restore New")
	}
}

func TestShortcutMapperRunsSavedMacro(t *testing.T) {
	state, driver := startDriver(t)
	state.namedMacros = []macro.Named{{Name: "Insert greeting", Steps: []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "hello"}}}}
	state.command(shortcutMapperCommand)
	requireEditorCommandFocus(t, driver, mapperSearchID)
	if err := driver.Type("Insert greeting"); err != nil {
		t.Fatal(err)
	}
	clickMapperRow(t, driver, "Macro: Insert greeting")
	clickID(t, driver, mapperCaptureID)
	if _, err := driver.ShortcutChord(primary(), 'j'); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, mapperApplyID)
	clickID(t, driver, mapperCloseID)
	requireEditorCommandFocus(t, driver, editorID)
	if _, err := driver.ShortcutChord(primary(), 'j'); err != nil || state.space.Current().Editor.String() != "hello" {
		t.Fatalf("saved macro did not run: %v, %q", err, state.space.Current().Editor.String())
	}
}

func clickMapperRow(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID.Scope() == mapperRows && hit.Label == name {
			clickID(t, driver, hit.ID)
			return
		}
	}
	t.Fatalf("mapper row %q is not visible", name)
}
