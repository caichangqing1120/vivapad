package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestMacroRecordsMultiSelectionAndTyping(t *testing.T) {
	state, driver := startDriver(t)
	state.macroPath = filepath.Join(t.TempDir(), "macro.json")
	doc := state.space.Current()
	doc.Editor.Insert("cat CAT cats cat")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Record Macro")
	state.command("Multi-select All")
	state.command("Multi-select All / Match Whole Word Only")
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	state.command("Stop Recording")
	if doc.Editor.String() != "x x cats x" || len(state.macroLast) != 2 {
		t.Fatalf("recording text=%q steps=%+v notice=%q", doc.Editor.String(), state.macroLast, state.notice)
	}
	steps, err := macro.Load(state.macroPath)
	if err != nil || len(steps) != 2 {
		t.Fatalf("stored steps=%+v: %v", steps, err)
	}
	const initial = "dog DOG dogs dog"
	path := filepath.Join(t.TempDir(), "target.txt")
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	target := state.space.Current()
	target.Editor.MoveTo(1, false)
	if !state.playMacroSteps(steps, 2, false) || target.Editor.String() != "x x dogs x" || len(state.multiSelections[0].ranges) != 3 {
		t.Fatalf("replay text=%q ranges=%+v notice=%q", target.Editor.String(), state.multiSelections[0].ranges, state.notice)
	}
	state.command("Undo")
	if target.Editor.String() != initial || target.Editor.CanUndo() || state.multiSelections[0].active {
		t.Fatalf("undo text=%q ranges=%+v history=%t", target.Editor.String(), state.multiSelections[0].ranges, target.Editor.CanUndo())
	}
}

func TestMacroRecordsMultiSelectionNavigationAndDelete(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat cat cat!")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	state.command("Record Macro")
	state.command("Multi-select Next / Match Case & Whole Word")
	state.command("Multi-select Next / Match Case & Whole Word")
	state.command("Skip Current Multi-Select")
	state.command("Undo Latest Multi-Select")
	if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	state.command("Stop Recording")
	flags := macro.SelectMatchCase | macro.SelectWholeWord
	want := []macro.Step{
		{Kind: macro.SelectionStep, Selection: macro.SelectNextMatch, SelectFlags: flags},
		{Kind: macro.SelectionStep, Selection: macro.SelectNextMatch, SelectFlags: flags},
		{Kind: macro.SelectionStep, Selection: macro.SkipCurrentMatch},
		{Kind: macro.SelectionStep, Selection: macro.UndoLatestSelection},
		{Kind: macro.MultiTextStep, TextKind: app.TextCommandEvent, Command: app.TextDeleteBackward},
	}
	if doc.Editor.String() != " cat cat cat!" || !reflect.DeepEqual(state.macroLast, want) {
		t.Fatalf("recording text=%q steps=%+v notice=%q", doc.Editor.String(), state.macroLast, state.notice)
	}
	doc = state.space.NewDocument()
	doc.Editor.Insert("dog dog dog dog!")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	if !state.playMacroSteps(want, 1, false) || doc.Editor.String() != " dog dog dog!" {
		t.Fatalf("replay text=%q notice=%q", doc.Editor.String(), state.notice)
	}
}

func TestMacroMultiSelectionFailurePreservesSource(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	const value = "a b aa z"
	doc.Editor.Insert(value)
	doc.Editor.ClearHistory()
	// The first selection step stages new ranges, but replay never reaches EOF.
	doc.Editor.MoveTo(0, false)
	steps := []macro.Step{{Kind: macro.SelectionStep, Selection: macro.SelectNextMatch, SelectFlags: macro.SelectWholeWord}}
	if state.playMacroSteps(steps, 1, true) || doc.Editor.String() != value || doc.Editor.Caret() != 0 || doc.Editor.CanUndo() || state.multiSelections[0].active {
		t.Fatalf("failed playback changed source: text=%q caret=%d ranges=%+v history=%t notice=%q",
			doc.Editor.String(), doc.Editor.Caret(), state.multiSelections[0].ranges, doc.Editor.CanUndo(), state.notice)
	}
}

func TestMacroMissingMultiSelectionRollsBackEarlierSteps(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert(" . ")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	steps := []macro.Step{
		{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "!"},
		{Kind: macro.SelectionStep, Selection: macro.SelectAllMatches, SelectFlags: macro.SelectWholeWord},
		{Kind: macro.MultiTextStep, TextKind: app.TextInsert, Text: "x"},
	}
	if state.playMacroSteps(steps, 1, false) || doc.Editor.String() != " . " || doc.Editor.Caret() != 1 || doc.Editor.CanUndo() || state.multiSelections[0].active || !strings.Contains(state.notice, "no active ranges") {
		t.Fatalf("failed playback changed source: text=%q caret=%d ranges=%+v history=%t notice=%q",
			doc.Editor.String(), doc.Editor.Caret(), state.multiSelections[0].ranges, doc.Editor.CanUndo(), state.notice)
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
