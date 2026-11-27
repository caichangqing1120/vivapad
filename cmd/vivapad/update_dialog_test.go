package main

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/update"
	"vivapad/internal/preferences"
	"vivapad/internal/updater"
)

func TestM6PreferencesDriver(t *testing.T) {
	state := newStartupModel(nil, filepath.Join(t.TempDir(), "session.json"))
	state.systemLanguages = func() []string { return []string{"en-US"} }
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	state.fontSize = 18
	state.command("Preferences...")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, preferencesGeneralID)
	clickID(t, driver, interfaceLanguageScope.At(1))
	if !state.chineseUI {
		t.Fatal("explicit Simplified Chinese did not override the system language")
	}
	clickID(t, driver, interfaceSizeScope.At(2))
	clickID(t, driver, tabNumbersID)
	clickID(t, driver, autoCheckUpdatesID)
	stored, err := preferences.Load(state.preferencesPath)
	if err != nil || stored.Language != "zh-CN" || stored.UITextSize != "large" || !stored.ShowTabNumbers || !stored.AutoCheckUpdates || stored.FontSize != 18 {
		t.Fatalf("settings = %+v, %v", stored, err)
	}
	clickID(t, driver, resetPreferencesID)
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || !state.preferencesOpen || !state.showTabNumbers {
		t.Fatalf("reset cancellation failed: %v", err)
	}
	clickID(t, driver, resetPreferencesID)
	clickID(t, driver, confirmResetPreferencesID)
	if state.showTabNumbers || state.autoCheckUpdates || state.language != "system" || state.uiTextSize != "medium" || state.chineseUI {
		t.Fatal("reset did not restore defaults")
	}
	clickID(t, driver, preferencesCloseID)
	state.command("New")
	state.command("New")
	if _, err := driver.ShortcutChord(primary(), '1'); err != nil || state.space.Active != 0 {
		t.Fatalf("first tab shortcut = %d, %v", state.space.Active, err)
	}
	if _, err := driver.ShortcutChord(primary(), '9'); err != nil || state.space.Active != 0 {
		t.Fatalf("out-of-range shortcut changed tab: %v", err)
	}
	clickID(t, driver, menuScope.At(12))
	clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[12], checkUpdatesCommand)))
	clickID(t, driver, copyDiagnosticsID)
	clipboard := driver.Clipboard()
	if !strings.Contains(clipboard, "Version: 0.1.0") || !strings.Contains(clipboard, "Viva:") || state.updates.copiedUntil.IsZero() {
		t.Fatalf("diagnostics = %q", clipboard)
	}
}

func TestM6SplitTabNumbersFollowVisibleOrder(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Name = "left"
	state.command("New")
	state.space.Current().Name = "right first"
	state.command("Split View")
	state.command("New")
	state.space.Current().Name = "right second"
	state.showTabNumbers = true
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.ShortcutChord(primary(), '1'); err != nil || state.activePane != 1 || state.space.Current().Name != "right first" {
		t.Fatalf("right tab 1 selected %s in pane %d: %v", state.space.Current().Name, state.activePane, err)
	}
	if _, err := driver.ShortcutChord(primary(), '2'); err != nil || state.activePane != 1 || state.space.Current().Name != "right second" {
		t.Fatalf("right tab 2 selected %s in pane %d: %v", state.space.Current().Name, state.activePane, err)
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.Role == gfx.RoleTab && hit.Label == "2  right second" && hit.Selected {
			return
		}
	}
	t.Fatal("right tab number and shortcut do not agree")
}

func TestM6SystemLanguageUsesPreferencesInOrder(t *testing.T) {
	state := newModel()
	state.systemLanguages = func() []string { return []string{"fr-FR", "zh-CN", "en"} }
	state.resolveInterfaceLanguage()
	if !state.chineseUI {
		t.Fatal("first supported system language was not selected")
	}
	state.language = "en"
	state.resolveInterfaceLanguage()
	if state.chineseUI {
		t.Fatal("explicit English did not override system language")
	}
}

func TestM6InstallDriverHonorsDirtyGate(t *testing.T) {
	for _, mode := range []string{"cancel", "managed", "save", "discard"} {
		t.Run(mode, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			path := filepath.Join(t.TempDir(), "saved.txt")
			if err := doc.Save(path); err != nil {
				t.Fatal(err)
			}
			doc.Editor.Insert("unsaved")
			calls, quits := 0, 0
			state.updates.engine = testM6Updater{install: func(quit func()) error {
				calls++
				if mode == "managed" {
					return update.ErrManagedExternally
				}
				quit()
				return nil
			}}
			state.updates.prepared = &updater.Prepared{}
			config := newConfig(state)
			state.closeWindow = func() { config.OnCloseRequest(func() { quits++ }) }
			state.openUpdatePreferences()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, installUpdateID)
			if calls != 0 || quits != 0 || state.pendingClose < 0 || !doc.Dirty() {
				t.Fatal("installer bypassed dirty document gate")
			}
			switch mode {
			case "cancel":
				clickID(t, driver, cancelCloseID)
				if calls != 0 || quits != 0 || !doc.Dirty() || !state.preferencesOpen {
					t.Fatal("cancel mutated the document or ran the installer")
				}
			case "managed":
				clickID(t, driver, discardID)
				if calls != 1 || quits != 0 || !doc.Dirty() || !errors.Is(state.updates.err, update.ErrManagedExternally) || !state.preferencesOpen {
					t.Fatal("managed installation lost the draft or failed to report")
				}
			case "save":
				clickID(t, driver, saveCloseID)
				if calls != 1 || quits != 1 || doc.Dirty() {
					t.Fatal("save did not finish before install and quit")
				}
			case "discard":
				clickID(t, driver, discardID)
				if calls != 1 || quits != 1 {
					t.Fatal("confirmed discard did not install and quit")
				}
			}
		})
	}
}

func TestM6UpdateCheckCanBeCanceledInDarkDriver(t *testing.T) {
	state, driver := startDriver(t)
	finished := make(chan struct{})
	state.updates.engine = testM6Updater{check: func(ctx context.Context) (*updater.Candidate, error) {
		defer close(finished)
		<-ctx.Done()
		return nil, nil
	}}
	state.themeChoice = preferences.ModernDark
	state.openUpdatePreferences()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Resize(940, 620); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("About & Updates layout: %+v", problems)
	}
	clickID(t, driver, checkUpdateID)
	clickID(t, driver, cancelUpdateID)
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("Cancel did not stop the check")
	}
	deadline := time.Now().Add(3 * time.Second)
	for state.updates.operation != nil && time.Now().Before(deadline) {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if !errors.Is(state.updates.err, context.Canceled) || state.updates.checked {
		t.Fatalf("canceled check accepted a result: %v", state.updates.err)
	}
}

type testM6Updater struct {
	check   func(context.Context) (*updater.Candidate, error)
	install func(func()) error
}

func (service testM6Updater) Check(ctx context.Context) (*updater.Candidate, error) {
	return service.check(ctx)
}
func (testM6Updater) Prepare(context.Context, *updater.Candidate, func(updater.Progress)) (*updater.Prepared, error) {
	return nil, nil
}
func (service testM6Updater) Install(_ *updater.Prepared, quit func()) error {
	return service.install(quit)
}
