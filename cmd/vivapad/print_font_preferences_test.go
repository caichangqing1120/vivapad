package main

import (
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"vivapad/internal/preferences"
	"vivapad/internal/printing"
)

func TestPrintFontPreferencesPersistAndReachJob(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	state := newStartupModel(nil, path)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.command("Preferences...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, preferencesPrintFontsID)
	clickID(t, driver, printFontSizeIncreaseScope.At(0))
	clickID(t, driver, printFontBoldScope.At(0))
	clickID(t, driver, printFontItalicScope.At(0))
	clickID(t, driver, printFontSizeDecreaseScope.At(1))
	clickID(t, driver, printFontItalicScope.At(1))
	if state.printSettings.HeaderFont != (printing.BandFont{Size: 10, Bold: true, Italic: true}) ||
		state.printSettings.FooterFont != (printing.BandFont{Size: 8, Italic: true}) {
		t.Fatalf("print font controls = %+v", state.printSettings)
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact print typography: %+v", problems)
	}
	stored, err := preferences.Load(filepath.Join(filepath.Dir(path), "settings.json"))
	if err != nil || stored.Print != state.printSettings {
		t.Fatalf("stored band fonts = %+v, %v", stored.Print, err)
	}
	restarted := newStartupModel(nil, path)
	var received printing.Job
	restarted.printJob = func(job printing.Job) (int, error) { received = job; return 1, nil }
	restarted.command("Print Now")
	if received.Settings.HeaderFont != state.printSettings.HeaderFont || received.Settings.FooterFont != state.printSettings.FooterFont {
		t.Fatalf("print job band fonts = %+v", received.Settings)
	}
}
