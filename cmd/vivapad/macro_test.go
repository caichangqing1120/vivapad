package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/macro"
)

func TestMacroRecordsCommittedIMEAndCodeTabThenReplaysAtCaret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, editorID)
	clickID(t, driver, menuScope.At(8))
	clickID(t, driver, menuItemScope.At(0))
	clickID(t, driver, editorID)
	if err := driver.Compose("zhong"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("中"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(8))
	clickID(t, driver, menuItemScope.At(1))
	if got := state.space.Current().Editor.String(); got != "中\t" {
		t.Fatalf("recorded editing = %q", got)
	}
	stored, err := macro.Load(filepath.Join(filepath.Dir(path), "macro.json"))
	if err != nil || len(stored) != 2 {
		t.Fatalf("stored macro = %+v, %v", stored, err)
	}
	doc := state.space.Current()
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	doc.Editor.Insert("prefixsuffix")
	doc.Editor.MoveTo(len([]rune("prefix")), false)
	clickID(t, driver, menuScope.At(8))
	clickID(t, driver, menuItemScope.At(2))
	if got := doc.Editor.String(); got != "prefix中\tsuffix" {
		t.Fatalf("macro replay at caret = %q", got)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "prefixsuffix" {
		t.Fatalf("macro undo = %q", doc.Editor.String())
	}
	restored := newStartupModel(nil, path)
	if len(restored.macroLast) != 2 {
		t.Fatalf("restart lost recorded macro: %+v", restored.macroLast)
	}
}

func TestMacroShortcutAndEmptyRecordingPreserveLastMacro(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'r'); err != nil || !handled {
		t.Fatalf("start recording handled=%v, %v", handled, err)
	}
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'r'); err != nil || !handled {
		t.Fatalf("stop recording handled=%v, %v", handled, err)
	}
	if len(state.macroLast) != 1 {
		t.Fatalf("recorded steps = %+v", state.macroLast)
	}
	state.command("Record Macro")
	state.command("Stop Recording")
	if len(state.macroLast) != 1 {
		t.Fatal("empty recording replaced the last macro")
	}
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'p'); err != nil || !handled {
		t.Fatalf("playback handled=%v, %v", handled, err)
	}
	if got := state.space.Current().Editor.String(); got != "xx" {
		t.Fatalf("keyboard macro playback = %q", got)
	}
}

func TestMacroRecordingStopsOnDocumentSwitchWithoutSavingPartialSteps(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	state.command("Record Macro")
	if err := driver.Type("secret"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, newID)
	if state.macroRecording || len(state.macroLast) != 0 || !strings.Contains(state.notice, "canceled") {
		t.Fatalf("cross-document recording = active:%v last:%+v notice:%q", state.macroRecording, state.macroLast, state.notice)
	}
}

func TestMacroCancelsOnUnrecordedCaretNavigation(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	state.command("Record Macro")
	if err := driver.Type("first\nsecond"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyUp, 0); err != nil {
		t.Fatal(err)
	}
	if state.macroRecording || len(state.macroLast) != 0 || !strings.Contains(state.notice, "movement") {
		t.Fatalf("unrecorded navigation left macro active: %v, %q", state.macroRecording, state.notice)
	}
}

func TestMacroClearRemovesSavedSteps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	state.command("Record Macro")
	state.recordMacro(state.space.Current(), macro.Step{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "x"})
	state.command("Stop Recording")
	state.command("Clear Macro")
	if len(state.macroLast) != 0 || len(newStartupModel(nil, path).macroLast) != 0 {
		t.Fatal("cleared macro was restored")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "macro.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clear left macro file: %v", err)
	}
}

func TestMacroRecordsCompositionCommittedByUnmark(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	state.command("Record Macro")
	if err := driver.Compose("中"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Compose(""); err != nil {
		t.Fatal(err)
	}
	state.command("Stop Recording")
	if len(state.macroLast) != 1 || state.macroLast[0].Text != "中" {
		t.Fatalf("committed composition steps = %+v", state.macroLast)
	}
}

func TestMacroCancelsWhenNativeSelectionChangesWithoutReplayStep(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	state.command("Record Macro")
	if err := driver.Type("abc"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(0, 2); err != nil {
		t.Fatal(err)
	}
	if state.macroRecording || len(state.macroLast) != 0 || !strings.Contains(state.notice, "selection") {
		t.Fatalf("unrecorded selection left macro active: %v, %q", state.macroRecording, state.notice)
	}
}

func TestMacroCancelsOnMouseCaretMoveAndSearch(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("first\nsecond"); err != nil {
		t.Fatal(err)
	}
	state.command("Record Macro")
	if err := driver.Click(83, menuHeight+toolbarHeight+tabHeight+15); err != nil {
		t.Fatal(err)
	}
	if state.macroRecording || !strings.Contains(state.notice, "selection") {
		t.Fatalf("mouse caret move remained recordable: active=%v notice=%q", state.macroRecording, state.notice)
	}
	state.command("Record Macro")
	state.command("Find...")
	if state.macroRecording || !strings.Contains(state.notice, "command") {
		t.Fatalf("search command remained recordable: active=%v notice=%q", state.macroRecording, state.notice)
	}
}

func TestMacroCancelsOnUndoFromExistingDocumentHistory(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("before"); err != nil {
		t.Fatal(err)
	}
	state.command("Record Macro")
	state.command("Undo")
	if state.macroRecording || len(state.macroLast) != 0 || !strings.Contains(state.notice, "canceled") {
		t.Fatalf("undo history was silently recorded: active=%v notice=%q", state.macroRecording, state.notice)
	}
}
