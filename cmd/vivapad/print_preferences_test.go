package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"vivapad/internal/preferences"
	"vivapad/internal/printing"
)

func TestPrintPreferencesCompactControlsPersistAndReachPrintJob(t *testing.T) {
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
	clickID(t, driver, preferencesPrintID)
	clickID(t, driver, printColorScope.At(2))
	clickID(t, driver, printLineNumbersID)
	clickID(t, driver, printFormFeedID)
	clickID(t, driver, printMarginIncrementScope.At(0))
	if state.printSettings.ColorMode != printing.ColorBlackOnWhite || state.printSettings.LineNumbers ||
		!state.printSettings.FormFeedPageBreak || state.printSettings.MarginMM.Top != 1 {
		t.Fatalf("print layout controls = %+v", state.printSettings)
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact print layout: %+v", problems)
	}
	clickID(t, driver, preferencesPrintBandsID)
	clickID(t, driver, printBandFieldScope.At(0))
	if err := driver.Type("Report "); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, printVariableButtonID)
	clickID(t, driver, printVariableScope.At(1))
	clickID(t, driver, printInsertVariableID)
	clickID(t, driver, printBandFieldScope.At(4))
	if err := driver.Type("Page "); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, printVariableButtonID)
	clickID(t, driver, printVariableScope.At(3))
	clickID(t, driver, printInsertVariableID)
	if state.printSettings.Header.Left != "Report $(FILE_NAME)" ||
		state.printSettings.Footer.Middle != "Page $(CURRENT_PRINTING_PAGE)" {
		t.Fatalf("band editor = %+v", state.printSettings)
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact print bands: %+v", problems)
	}
	stored, err := preferences.Load(filepath.Join(filepath.Dir(path), "settings.json"))
	if err != nil || stored.Print != state.printSettings {
		t.Fatalf("stored print settings = %+v, %v", stored.Print, err)
	}
	clickID(t, driver, preferencesCloseID)
	restarted := newStartupModel(nil, path)
	var received printing.Job
	restarted.printJob = func(job printing.Job) (int, error) { received = job; return 1, nil }
	restarted.command("Print Now")
	if received.Settings != stored.Print || !strings.Contains(restarted.notice, "Submitted") {
		t.Fatalf("restarted print settings = %+v, notice=%q", received.Settings, restarted.notice)
	}
}

func TestCapturePrintPreferencesWhenRequested(t *testing.T) {
	directory := os.Getenv("VIVAPAD_PRINT_PREFERENCES_SHOTS")
	if directory == "" {
		t.Skip("set VIVAPAD_PRINT_PREFERENCES_SHOTS to capture print settings")
	}
	state := newModel()
	state.printSettings.Header.Left = "Report $(FILE_NAME)"
	state.printSettings.Footer.Middle = "Page $(CURRENT_PRINTING_PAGE)"
	state.printSettings.HeaderFont = printing.BandFont{Size: 14, Bold: true, Italic: true}
	state.printSettings.FooterFont = printing.BandFont{Size: 8, Italic: true}
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
		clickID(t, session.Driver(), preferencesPrintID)
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s Print preferences layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(directory, "print-preferences-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), preferencesPrintBandsID)
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s Header/Footer preferences layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(directory, "print-bands-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), preferencesPrintFontsID)
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s Print Fonts layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(directory, "print-fonts-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPrintVariableInsertKeepsDraftOnStorageFailure(t *testing.T) {
	state, driver := startDriver(t)
	blocked := filepath.Join(t.TempDir(), "settings.json")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	state.preferencesPath = blocked
	state.command("Preferences...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, preferencesPrintBandsID)
	clickID(t, driver, printBandFieldScope.At(0))
	if err := driver.Type("Page "); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, printVariableButtonID)
	clickID(t, driver, printVariableScope.At(3))
	clickID(t, driver, printInsertVariableID)
	want := "Page $(CURRENT_PRINTING_PAGE)"
	if state.printSettings.Header.Left != want || state.printFields[0].String() != want || !strings.Contains(state.notice, "Settings not saved") {
		t.Fatalf("failed save diverged field and setting: field=%q setting=%q notice=%q", state.printFields[0].String(), state.printSettings.Header.Left, state.notice)
	}
}
