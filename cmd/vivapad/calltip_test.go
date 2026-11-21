package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestParameterHintShowsAndCyclesWithoutEditing(t *testing.T) {
	state, driver := startDriver(t)
	root := t.TempDir()
	state.preferencesPath = filepath.Join(root, "settings.json")
	if err := os.Mkdir(filepath.Join(root, "apis"), 0700); err != nil {
		t.Fatal(err)
	}
	const data = `<NotepadPlus><AutoComplete><KeyWord name="demo" func="yes"><Overload><Param name="first"/><Param name="second"/></Overload><Overload><Param name="left"/><Param name="right"/></Overload></KeyWord></AutoComplete></NotepadPlus>`
	if err := os.WriteFile(filepath.Join(root, "apis", "go.xml"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.languages[doc] = ".go"
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	const original = "demo(1,\n  "
	if err := driver.Type(original); err != nil {
		t.Fatal(err)
	}
	openMenu := func(index int) {
		clickID(t, driver, menuScope.At(1))
		clickID(t, driver, menuItemScope.At(18))
		clickID(t, driver, autoCompletionScope.At(index))
	}
	openMenu(2)
	if hint := state.calltip; hint == nil || hint.call.Argument != 1 || hint.overload != 0 {
		t.Fatalf("initial hint = %+v, %s", hint, state.notice)
	}
	openMenu(4)
	if hint := state.calltip; hint == nil || hint.overload != 1 {
		t.Fatalf("next hint = %+v", hint)
	}
	openMenu(3)
	if hint := state.calltip; hint == nil || hint.overload != 0 {
		t.Fatalf("previous hint = %+v", hint)
	}
	clickID(t, driver, calltipPreviousID)
	if hint := state.calltip; hint == nil || hint.overload != 1 {
		t.Fatalf("previous arrow did not wrap to the last overload: %+v", hint)
	}
	requireEditorCommandFocus(t, driver, editorID)
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.calltip != nil || doc.Editor.String() != original {
		t.Fatalf("Escape left hint %v or changed %q: %v", state.calltip, doc.Editor.String(), err)
	}
	openMenu(2)
	state.languages[doc] = ".py"
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil || state.calltip != nil {
		t.Fatalf("hint survived language change: %v, %v", state.calltip, err)
	}
	state.languages[doc] = ".go"
	openMenu(2)
	if state.calltip == nil {
		t.Fatal("hint did not reopen")
	}
	requireEditorCommandFocus(t, driver, editorID)
	if err := driver.Type(")"); err != nil || state.calltip != nil || doc.Editor.String() != original+")" {
		t.Fatalf("closing parenthesis left hint %v or lost typing: %q, %v", state.calltip, doc.Editor.String(), err)
	}
}

func TestParameterHintRefusesAndDismissesMultipleCarets(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	state.languages[doc] = ".go"
	const source = "append([]int{1}, "
	doc.Editor.Insert(source)
	doc.Editor.ClearHistory()
	last := doc.Editor.RuneCount()
	addCaret := func() {
		doc.Editor.SetSelections([]text.Range{{Start: 0, End: 0}, {Start: last, End: last}}, 1)
		if doc.Editor.SelectionCount() != 2 || doc.Editor.Caret() != last {
			t.Fatal("invalid test setup: primary and secondary carets were not set")
		}
	}
	addCaret()
	state.command(parameterHintCommand)
	if state.calltip != nil {
		t.Error("parameter hint opened for two carets")
	}
	doc.Editor.ClearSecondarySelections()
	state.command(parameterHintCommand)
	if hint := state.calltip; hint == nil || hint.call.Name != "append" || hint.call.Argument != 1 {
		t.Fatalf("expected the second argument of append at a single caret, got %+v", hint)
	}
	addCaret()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.calltip != nil {
		t.Error("parameter hint survived adding a second caret without changing the primary")
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == calltipLabelID || hit.ID == calltipArgumentID {
			t.Fatal("a stale hint remains in the rendered view")
		}
	}
	if doc.Editor.String() != source || doc.Editor.SelectionCount() != 2 || doc.Editor.CanUndo() {
		t.Fatal("parameter hint altered the buffer, selections or undo history")
	}
}

func TestParameterHintIgnoresNestedBlockComments(t *testing.T) {
	for _, tc := range []struct {
		mode, catalog string
		width, height int
	}{
		{".rs", "rust", 640, 400},
		{".swift", "swift", 1280, 800},
	} {
		t.Run(tc.catalog, func(t *testing.T) {
			state, driver := startDriver(t)
			root := t.TempDir()
			state.preferencesPath = filepath.Join(root, "settings.json")
			if err := os.Mkdir(filepath.Join(root, "apis"), 0700); err != nil {
				t.Fatal(err)
			}
			const catalog = `<NotepadPlus><AutoComplete><KeyWord name="demo" func="yes"><Overload><Param name="first"/><Param name="second"/></Overload></KeyWord></AutoComplete></NotepadPlus>`
			if err := os.WriteFile(filepath.Join(root, "apis", tc.catalog+".xml"), []byte(catalog), 0600); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			state.languages[doc] = tc.mode
			if err := driver.Resize(tc.width, tc.height); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, editorID)
			const source = "demo(first, /* outer\n /* inner */ ), demo(, */ "
			if err := driver.Type(source); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(1))
			clickID(t, driver, menuItemScope.At(18))
			clickID(t, driver, autoCompletionScope.At(2))
			if hint := state.calltip; hint == nil || hint.call.Name != "demo" || hint.call.Open != 4 || hint.call.Argument != 1 {
				t.Fatalf("want second parameter of outer call, got %+v (%s)", hint, state.notice)
			}
			if doc.Editor.String() != source {
				t.Fatal("showing parameters modified the document")
			}
			requireEditorCommandFocus(t, driver, editorID)
		})
	}
}

func TestParameterHintIgnoresMultilineStringDelimiters(t *testing.T) {
	for _, tc := range []struct {
		language, mode, source string
	}{
		{"rust", ".rs", "demo(\"one\n), demo(, \\\"quoted\\\"\nlast\", "},
		{"swift", ".swift", "demo(\"\"\"\n), demo(, \"quoted\"\n\"\"\", "},
	} {
		t.Run(tc.language, func(t *testing.T) {
			state, driver := startDriver(t)
			root := t.TempDir()
			state.preferencesPath = filepath.Join(root, "settings.json")
			api := filepath.Join(root, "apis")
			if err := os.Mkdir(api, 0700); err != nil {
				t.Fatal(err)
			}
			const catalog = `<NotepadPlus><AutoComplete><KeyWord name="demo" func="yes"><Overload><Param name="first"/><Param name="second"/></Overload></KeyWord></AutoComplete></NotepadPlus>`
			if err := os.WriteFile(filepath.Join(api, tc.language+".xml"), []byte(catalog), 0600); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			state.languages[doc] = tc.mode
			clickID(t, driver, editorID)
			if err := driver.Type(tc.source); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(1))
			clickID(t, driver, menuItemScope.At(18))
			clickID(t, driver, autoCompletionScope.At(2))
			if hint := state.calltip; hint == nil || hint.call.Open != 4 || hint.call.Name != "demo" || hint.call.Argument != 1 {
				t.Fatalf("want second parameter of the outer call, got %+v (%s)", hint, state.notice)
			}
			if doc.Editor.String() != tc.source || doc.Editor.Caret() != len(tc.source) {
				t.Fatal("showing the hint changed text or moved the caret")
			}
		})
	}
}
