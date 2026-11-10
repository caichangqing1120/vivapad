package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/macro"
)

func clickLabeledControl(t *testing.T, driver *app.Driver, label string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label {
			clickID(t, driver, hit.ID)
			return
		}
	}
	t.Fatalf("control %q not found", label)
}

func TestSaveNamedMacroDialogPersistsAndMenuReplays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	state.command("Record Macro")
	state.recordMacro(state.space.Current(), macro.Step{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "!"})
	state.command("Stop Recording")
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(8))
	clickLabeledControl(t, driver, "Save Current Recorded Macro...")
	clickLabeledControl(t, driver, "Macro name")
	if err := driver.Type("Emphasis"); err != nil {
		t.Fatal(err)
	}
	clickLabeledControl(t, driver, "Save macro")
	named, err := macro.LoadNamed(filepath.Join(filepath.Dir(path), "named-macros.json"))
	if err != nil || len(named) != 1 || named[0].Name != "Emphasis" || len(named[0].Steps) != 1 {
		t.Fatalf("saved named macro = %+v, %v", named, err)
	}
	restarted := newStartupModel(nil, path)
	restartedDriver, err := app.NewDriver(newConfig(restarted), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restartedDriver.Close()
	clickID(t, restartedDriver, menuScope.At(8))
	clickLabeledControl(t, restartedDriver, "Saved Macros")
	clickLabeledControl(t, restartedDriver, "Emphasis")
	if got := restarted.space.Current().Editor.String(); got != "!" {
		t.Fatalf("saved macro replay = %q, notice=%q", got, restarted.notice)
	}
	if !restarted.space.Current().Editor.Undo() {
		t.Fatal("saved macro replay was not undoable")
	}
}

func TestRunMultipleMacroDialogUsesCountAndEOFWithoutPartialEdits(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("abc")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.macroLast = []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "!"},
		{Kind: macro.TextStep, TextKind: app.TextCommandEvent, Command: app.TextMoveRight}}
	clickID(t, driver, menuScope.At(8))
	clickLabeledControl(t, driver, "Run a Macro Multiple Times...")
	clickLabeledControl(t, driver, "Repeat count")
	if handled, err := driver.ShortcutChord(primary(), 'a'); err != nil || !handled {
		t.Fatalf("select count = %v, %v", handled, err)
	}
	if err := driver.Type("2"); err != nil {
		t.Fatal(err)
	}
	clickLabeledControl(t, driver, "Run macro")
	if got := doc.Editor.String(); got != "!a!bc" {
		t.Fatalf("two runs = %q, notice=%q", got, state.notice)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "abc" {
		t.Fatal("multiple runs did not undo together")
	}
	doc.Editor.MoveTo(0, false)
	clickID(t, driver, menuScope.At(8))
	clickLabeledControl(t, driver, "Run a Macro Multiple Times...")
	clickLabeledControl(t, driver, "Run until end of file")
	clickLabeledControl(t, driver, "Run macro")
	if got := doc.Editor.String(); got != "!a!b!c" {
		t.Fatalf("EOF run = %q, notice=%q", got, state.notice)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "abc" {
		t.Fatal("EOF run did not undo together")
	}
}

func TestMacroDialogCancelAndReadOnlySplitPreserveDocuments(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	state.macroLast = []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "!"}}
	clickID(t, driver, menuScope.At(8))
	clickLabeledControl(t, driver, "Run a Macro Multiple Times...")
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if left.Editor.String() != "left" || right.Editor.String() != "right" {
		t.Fatal("cancel edited a split document")
	}
	state.readOnly[right] = true
	clickID(t, driver, menuScope.At(8))
	clickLabeledControl(t, driver, "Run a Macro Multiple Times...")
	clickLabeledControl(t, driver, "Run macro")
	if right.Editor.String() != "right" || left.Editor.String() != "left" || !strings.Contains(state.notice, "Read-only") {
		t.Fatalf("read-only run edited a pane: left=%q right=%q notice=%q", left.Editor.String(), right.Editor.String(), state.notice)
	}
	if selection := right.Editor.Selection(); selection != (text.Range{Start: 5, End: 5}) {
		t.Fatalf("read-only run moved caret: %+v", selection)
	}
}

func TestNamedMacroPickerAndInvalidRepeatKeepSourceUntouched(t *testing.T) {
	state, driver := startDriver(t)
	state.macroLast = []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "X"}}
	state.namedMacros = []macro.Named{{Name: "Named Y", Steps: []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "Y"}}}}
	doc := state.space.Current()
	clickID(t, driver, menuScope.At(8))
	clickLabeledControl(t, driver, "Run a Macro Multiple Times...")
	clickLabeledControl(t, driver, "Macro to run")
	clickLabeledControl(t, driver, "Named Y")
	clickLabeledControl(t, driver, "Repeat count")
	if handled, err := driver.ShortcutChord(primary(), 'a'); err != nil || !handled {
		t.Fatalf("select count = %v, %v", handled, err)
	}
	if err := driver.Type("0"); err != nil {
		t.Fatal(err)
	}
	clickLabeledControl(t, driver, "Run macro")
	if !state.macroRunOpen || doc.Editor.String() != "" {
		t.Fatal("invalid count closed dialog or changed the document")
	}
	clickLabeledControl(t, driver, "Repeat count")
	if handled, err := driver.ShortcutChord(primary(), 'a'); err != nil || !handled {
		t.Fatalf("reselect count = %v, %v", handled, err)
	}
	if err := driver.Type("2"); err != nil {
		t.Fatal(err)
	}
	clickLabeledControl(t, driver, "Run macro")
	if got := doc.Editor.String(); got != "YY" || len(state.macroLast) != 1 || state.macroLast[0].Text != "X" {
		t.Fatalf("named macro run = %q, last=%+v", got, state.macroLast)
	}
}

func TestNamedMacroMenuScrollsAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	for index := range 22 {
		state.namedMacros = append(state.namedMacros, macro.Named{Name: "Macro " + strconv.Itoa(index), Steps: []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "!"}}})
	}
	clickID(t, driver, menuScope.At(8))
	clickLabeledControl(t, driver, "Saved Macros")
	seen := make(map[string]bool)
	for range len(state.namedMacros) {
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID.Scope() == savedMacroScope {
				seen[hit.Label] = true
				if hit.Bounds.X < 0 || hit.Bounds.Y < menuHeight || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("saved macro clipped: %+v", hit.Bounds)
				}
			}
		}
		if len(seen) == len(state.namedMacros) {
			break
		}
		clickLabeledControl(t, driver, "Next saved macros")
	}
	if len(seen) != len(state.namedMacros) {
		t.Fatalf("reachable saved macros = %d, want %d", len(seen), len(state.namedMacros))
	}
	clickLabeledControl(t, driver, "Macro 21")
	if state.space.Current().Editor.String() != "!" {
		t.Fatal("last saved macro was not runnable")
	}
}

func TestDuplicateNamedMacroAndEOFNoProgressAreAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	state.macroLast = []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "!"}}
	state.namedMacros = []macro.Named{{Name: "Existing", Steps: state.macroLast}}
	if err := macro.SaveNamed(state.namedMacroPath, state.namedMacros); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(state.namedMacroPath)
	if err != nil {
		t.Fatal(err)
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(8))
	clickLabeledControl(t, driver, "Save Current Recorded Macro...")
	clickLabeledControl(t, driver, "Macro name")
	if err := driver.Type("existing"); err != nil {
		t.Fatal(err)
	}
	clickLabeledControl(t, driver, "Save macro")
	data, err := os.ReadFile(state.namedMacroPath)
	if err != nil || string(data) != string(before) || !state.macroSaveOpen || len(state.namedMacros) != 1 {
		t.Fatalf("duplicate macro changed store = %q, %v", data, err)
	}
	clickLabeledControl(t, driver, "Cancel")
	doc := state.space.Current()
	doc.Editor.Insert("abc")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	clickID(t, driver, menuScope.At(8))
	clickLabeledControl(t, driver, "Run a Macro Multiple Times...")
	clickLabeledControl(t, driver, "Run until end of file")
	clickLabeledControl(t, driver, "Run macro")
	if doc.Editor.String() != "abc" || doc.Editor.Caret() != 0 || doc.Editor.CanUndo() || !state.macroRunOpen || !strings.Contains(state.notice, "advance") {
		t.Fatalf("non-progress EOF changed source = %q caret=%d notice=%q", doc.Editor.String(), doc.Editor.Caret(), state.notice)
	}
}
