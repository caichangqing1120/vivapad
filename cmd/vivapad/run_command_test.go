package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"vivapad/internal/runcommand"
)

func TestRunProgramChooserReturnsToCommandField(t *testing.T) {
	for _, trial := range []struct {
		name string
		err  error
	}{
		{"cancel", app.ErrCanceled},
		{"error", errors.New("program chooser failed")},
		{"choose", nil},
	} {
		t.Run(trial.name, func(t *testing.T) {
			state, driver := startDriver(t)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			doc.Editor.Insert("document")
			state.runLauncher = func(runcommand.Prepared) error {
				t.Error("choosing a program launched it")
				return nil
			}
			clickID(t, driver, menuScope.At(9))
			clickID(t, driver, runOpenID)
			if err := driver.Type("args"); err != nil || state.runInput.String() != "args" {
				t.Fatalf("initial command = %q, %v", state.runInput.String(), err)
			}
			state.runInput.SetSelection(text.Range{Start: 0, End: 4})
			path := filepath.Join(t.TempDir(), "program with space")
			if err := driver.QueueOpenFiles([]string{path}, trial.err); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, runDialogBrowseID)
			if req, ok := driver.LastOpenFilesRequest(); !ok || req.Title != "Program to Run" || req.AllowMultiple || !state.runDialogOpen {
				t.Fatalf("unexpected picker request or dialog state: %+v, present=%v", req, ok)
			}
			if got := driver.Snapshot().Focused; got != runDialogInputID {
				t.Fatalf("focus after chooser = %v, want Run command field", got)
			}
			want := "x"
			if trial.err == nil {
				want = strconv.Quote(path) + "x"
			}
			if err := driver.Type("x"); err != nil || state.runInput.String() != want || doc.Editor.String() != "document" {
				t.Fatalf("first key = %q, want %q; document=%q, %v", state.runInput.String(), want, doc.Editor.String(), err)
			}
		})
	}
}

func TestRunDialogCancelFailureAndExplicitLaunch(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("draft")
	doc.Editor.ClearHistory()
	var launched []runcommand.Prepared
	state.runLauncher = func(prepared runcommand.Prepared) error {
		launched = append(launched, prepared)
		return nil
	}
	clickID(t, driver, menuScope.At(9))
	clickID(t, driver, runOpenID)
	if !state.runDialogOpen {
		t.Fatal("Run command did not open a dialog")
	}
	clickID(t, driver, runDialogInputID)
	if err := driver.Type(`echo "$(CURRENT_WORD)"`); err != nil {
		t.Fatal(err)
	}
	if len(launched) != 0 {
		t.Fatal("typing in Run dialog launched a process")
	}
	clickID(t, driver, runDialogCancelID)
	if state.runDialogOpen || len(launched) != 0 || doc.Editor.String() != "draft" {
		t.Fatal("Cancel ran a command or modified the document")
	}
	state.command("Run...")
	state.runInput.SetSelection(text.Range{Start: 0, End: state.runInput.RuneCount()})
	state.runInput.Insert(`"unclosed`)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, runDialogExecuteID)
	if !state.runDialogOpen || state.runDialogError == "" || len(launched) != 0 {
		t.Fatal("invalid Run command closed dialog or launched")
	}
	state.runInput.SetSelection(text.Range{Start: 0, End: state.runInput.RuneCount()})
	state.runInput.Insert(`echo "$(CURRENT_WORD)"`)
	clickID(t, driver, runDialogExecuteID)
	if state.runDialogOpen || len(launched) != 1 || launched[0].Executable != "echo" || !reflect.DeepEqual(launched[0].Args, []string{"draft"}) || doc.Editor.String() != "draft" || doc.Editor.CanUndo() {
		t.Fatalf("Run did not launch safely: %+v, text=%q", launched, doc.Editor.String())
	}
}

func TestRunNamedCommandPersistsAndExpandsActiveFileOnReplay(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "first file.txt")
	if err := os.WriteFile(file, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{file}, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Run...")
	state.runInput.Insert(`echo "$(FULL_CURRENT_PATH)"`)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, runDialogSaveID)
	clickID(t, driver, runDialogNameID)
	if err := driver.Type("Show path"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, runDialogSaveID)
	if state.runDialogOpen || len(state.namedRuns) != 1 {
		t.Fatal("Run command did not save its template")
	}
	stored, err := runcommand.Load(filepath.Join(root, "settings", "run.json"))
	if err != nil || len(stored) != 1 || stored[0].Template != `echo "$(FULL_CURRENT_PATH)"` {
		t.Fatalf("stored Run template = %+v, %v", stored, err)
	}
	restarted := newStartupModel([]string{file}, sessionPath)
	replayed, nextDriver := []runcommand.Prepared{}, (*app.Driver)(nil)
	restarted.runLauncher = func(prepared runcommand.Prepared) error { replayed = append(replayed, prepared); return nil }
	nextDriver, err = app.NewDriver(newConfig(restarted), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer nextDriver.Close()
	clickID(t, nextDriver, menuScope.At(9))
	clickID(t, nextDriver, runNamedScope.At(0))
	if len(replayed) != 1 || !reflect.DeepEqual(replayed[0].Args, []string{restarted.space.Current().Path}) || restarted.space.Current().Dirty() {
		t.Fatalf("saved command replay = %+v", replayed)
	}
}

func TestRunVariablesAndDialogFitCompactWindow(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(9))
	clickID(t, driver, runOpenID)
	clickID(t, driver, runDialogVariablesID)
	seen := make([]bool, 11)
	for _, hit := range driver.Snapshot().Hits {
		for index := range seen {
			if hit.ID == runVariableScope.At(index) {
				seen[index] = true
				if hit.Bounds.X < 0 || hit.Bounds.Y < menuHeight || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("Run variable %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
	}
	for index, found := range seen {
		if !found {
			t.Fatalf("Run variable %d not reachable", index)
		}
	}
	clickID(t, driver, runVariableScope.At(0))
	if !strings.Contains(state.runInput.String(), "$(FULL_CURRENT_PATH)") || state.runVariablesOpen {
		t.Fatal("variable picker failed to insert original token")
	}
	clickID(t, driver, runDialogCancelID)
	if state.runDialogOpen || state.space.Current().Dirty() {
		t.Fatal("canceled variable dialog changed editor")
	}
}

func TestRunSaveWithoutPrivateStorageDoesNotClaimPersistence(t *testing.T) {
	state, driver := startDriver(t)
	state.command("Run...")
	state.runInput.Insert("echo value")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, runDialogSaveID)
	state.runName.Insert("Saved command")
	clickID(t, driver, runDialogSaveID)
	if !state.runDialogOpen || state.runDialogError == "" || len(state.namedRuns) != 0 {
		t.Fatal("command appeared saved without a writable private store")
	}
}

func TestRunMenuScrollAndReplayLastSavedCommand(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 40; index++ {
		state.namedRuns = append(state.namedRuns, runcommand.Named{Name: "Command " + strings.Repeat("x", index%3) + string(rune('A'+index)), Template: "echo value"})
	}
	var launched []runcommand.Prepared
	state.runLauncher = func(prepared runcommand.Prepared) error { launched = append(launched, prepared); return nil }
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(9))
	if state.runMenuVisible >= len(state.namedRuns)+2 {
		t.Fatal("long Run menu was not bounded")
	}
	clickID(t, driver, runMenuNextID)
	if state.runMenuOffset != 1 {
		t.Fatalf("Run menu scroll offset = %d", state.runMenuOffset)
	}
	state.runMenuOffset = len(state.namedRuns) + 2 - state.runMenuVisible
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == runNamedScope.At(len(state.namedRuns)) && !hit.Disabled {
			t.Fatal("Validate shortcuts.xml should remain disabled")
		}
	}
	clickID(t, driver, runNamedScope.At(len(state.namedRuns)-1))
	if len(launched) != 1 || launched[0].Executable != "echo" || !reflect.DeepEqual(launched[0].Args, []string{"value"}) || state.menuOpen != -1 {
		t.Fatalf("last Run command replay = %+v", launched)
	}
}

func TestSavedRunMenuReturnsFocusToActiveEditor(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "failure", err: errors.New("program unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, driver := startDriver(t)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			left := state.space.Current()
			left.Editor.Insert("left")
			right := state.space.NewDocument()
			right.Editor.Insert("right")
			right.Editor.ClearHistory()
			state.toggleSplit()
			state.namedRuns = []runcommand.Named{{Name: "Example", Template: "echo $(CURRENT_WORD)"}}
			calls := 0
			state.runLauncher = func(prepared runcommand.Prepared) error {
				calls++
				if !reflect.DeepEqual(prepared.Args, []string{"igh"}) {
					t.Fatalf("command used another pane or selection: %+v", prepared)
				}
				return tc.err
			}
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, rightEditorID)
			right.Editor.MoveTo(4, false)
			right.Editor.MoveTo(1, true)
			selection, caret := right.Editor.Selection(), right.Editor.Caret()
			clickID(t, driver, menuScope.At(9))
			clickID(t, driver, runNamedScope.At(0))
			if calls != 1 || state.activePane != 1 || state.space.Current() != right ||
				right.Editor.String() != "right" || right.Editor.Selection() != selection || right.Editor.Caret() != caret || right.Editor.CanUndo() {
				t.Fatal("running a saved command changed the active view or its editing state")
			}
			if tc.err != nil && !strings.Contains(state.notice, tc.err.Error()) {
				t.Fatalf("failed run notice = %q", state.notice)
			}
			requireEditorCommandFocus(t, driver, rightEditorID)
			if err := driver.Type("x"); err != nil || right.Editor.String() != "rxt" || left.Editor.String() != "left" {
				t.Fatalf("first key after running: left=%q right=%q err=%v", left.Editor.String(), right.Editor.String(), err)
			}
		})
	}
}

func TestRunDialogEscapeAndDuplicateNameLeavePriorSaveUntouched(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.json")
	state := newStartupModel(nil, path)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Run...")
	state.runInput.Insert("echo first")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, runDialogSaveID)
	state.runName.Insert("Existing")
	clickID(t, driver, runDialogSaveID)
	state.command("Run...")
	state.runInput.SetSelection(text.Range{Start: 0, End: state.runInput.RuneCount()})
	state.runInput.Insert("echo second")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, runDialogSaveID)
	state.runName.Insert("existing")
	clickID(t, driver, runDialogSaveID)
	if !state.runDialogOpen || state.runDialogError == "" || len(state.namedRuns) != 1 {
		t.Fatal("duplicate Run name replaced an existing command")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if !state.runDialogOpen || state.runSaveMode {
		t.Fatal("Escape did not return from named-command form")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.runDialogOpen {
		t.Fatal("Escape did not cancel Run dialog")
	}
	stored, err := runcommand.Load(filepath.Join(root, "run.json"))
	if err != nil || len(stored) != 1 || stored[0].Template != "echo first" {
		t.Fatalf("saved command after cancellation = %+v, %v", stored, err)
	}
}

func TestCaptureRunDialogWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RUN_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RUN_SHOTS to capture Run dialog")
	}
	state := newModel()
	state.command("Run...")
	state.runInput.Insert(`echo "$(FULL_CURRENT_PATH)"`)
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
			clickID(t, session.Driver(), runDialogVariablesID)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "run-dialog-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCaptureRunMenuWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RUN_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RUN_SHOTS to capture Run menu")
	}
	state := newModel()
	for index := 0; index < 25; index++ {
		state.namedRuns = append(state.namedRuns, runcommand.Named{Name: "Saved command " + string(rune('A'+index)), Template: "echo value"})
	}
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), menuScope.At(9))
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("Run menu layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "run-menu-compact.png")); err != nil {
		t.Fatal(err)
	}
}
