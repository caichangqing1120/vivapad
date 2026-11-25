package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"vivapad/internal/runcommand"
)

func TestWindowsRunEnvironmentFailureAndSavedReplay(t *testing.T) {
	const key = "VIVAPAD_RUN_DIALOG_TEST_VALUE"
	const template = "tool.exe %" + key + "%"
	t.Setenv(key, "initial")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	var calls []runcommand.Prepared
	state.runLauncher = func(prepared runcommand.Prepared) error {
		calls = append(calls, prepared)
		return nil
	}
	state.openRunDialog()
	state.runInput.Insert(template)
	state.applyRunDialog()
	if !state.runDialogOpen || state.runInput.String() != template || len(calls) != 0 || !strings.Contains(state.runDialogError, key) {
		t.Fatalf("undefined environment variable did not keep the Run draft: launched=%v, error=%q", calls, state.runDialogError)
	}
	state.closeRunDialog()
	state.namedRuns = []runcommand.Named{{Name: "Environment fixture", Template: template}}
	const value = `new value & "quoted"`
	if err := os.Setenv(key, value); err != nil {
		t.Fatal(err)
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(9))
	clickID(t, driver, runNamedScope.At(0))
	if len(calls) != 1 || calls[0].Executable != "tool.exe" || !slices.Equal(calls[0].Args, []string{value}) || state.namedRuns[0].Template != template {
		t.Fatalf("saved replay = %+v; want one literal argument and the unchanged template", calls)
	}
}
