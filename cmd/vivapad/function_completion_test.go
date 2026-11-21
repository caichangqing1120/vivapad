package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestFunctionCompletionUsesActiveLanguageAndUndo(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.go")
	if err := os.WriteFile(path, []byte("package example\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, filepath.Join(root, "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("appTail")
	doc.Editor.MoveTo(doc.Editor.RuneCount()-4, false)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(18))
	clickID(t, driver, autoCompletionScope.At(0))
	requireEditorCommandFocus(t, driver, editorID)
	if got := doc.Editor.String(); got != "package example\nappend" {
		t.Fatalf("built-in completion = %q", got)
	}
	if _, err := driver.ShortcutChord(primary(), 'z'); err != nil || doc.Editor.String() != "package example\nappTail" {
		t.Fatalf("Undo = %q, %v", doc.Editor.String(), err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "package example\r\n" {
		t.Fatalf("source bytes = %q, %v", data, err)
	}
}

func TestFunctionCompletionFiltersOverrideAndRefusesStaleOrComposingEditor(t *testing.T) {
	state, driver := startDriver(t)
	dir := t.TempDir()
	state.preferencesPath = filepath.Join(dir, "settings.json")
	if err := os.Mkdir(filepath.Join(dir, "apis"), 0700); err != nil {
		t.Fatal(err)
	}
	const catalog = `<NotepadPlus><AutoComplete><Environment additionalWordChar="."/><KeyWord name="lib.Clone" func="yes"/><KeyWord name="lib.Copy" func="yes"/></AutoComplete></NotepadPlus>`
	if err := os.WriteFile(filepath.Join(dir, "apis", "go.xml"), []byte(catalog), 0600); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.languages[doc] = ".go"
	clickID(t, driver, editorID)
	if err := driver.Type("lib.C"); err != nil {
		t.Fatal(err)
	}
	state.command(functionCompletionCommand)
	if state.wordCompletion == nil || len(state.wordCompletion.items) != 2 {
		t.Fatalf("override candidates = %+v, %q", state.wordCompletion, state.notice)
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("o"); err != nil {
		t.Fatal(err)
	}
	if state.wordCompletion == nil || len(state.wordCompletion.items) != 1 || state.wordCompletion.items[0] != "lib.Copy" {
		t.Fatalf("filtered candidates = %+v", state.wordCompletion)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil || doc.Editor.String() != "lib.Copy" {
		t.Fatalf("accept = %q, %v", doc.Editor.String(), err)
	}
	state.command("Undo")
	if doc.Editor.String() != "lib.Co" {
		t.Fatal("Undo did not restore the typed prefix")
	}
	doc.Editor.MoveTo(5, false)
	state.command(functionCompletionCommand)
	if state.wordCompletion == nil {
		t.Fatal("expected a popup before changing the language")
	}
	state.languages[doc] = ".py"
	state.acceptWordCompletion(0)
	if doc.Editor.String() != "lib.Co" {
		t.Fatal("stale catalog edited the document")
	}
	state.languages[doc] = ".go"
	state.readOnly[doc] = true
	state.command(functionCompletionCommand)
	if state.wordCompletion != nil || !strings.Contains(state.notice, "Read-only") {
		t.Fatalf("read-only: %+v, %s", state.wordCompletion, state.notice)
	}
	delete(state.readOnly, doc)
	doc.Editor.SetMarkedText("x", text.Range{Start: 1, End: 1})
	before := doc.Editor.String()
	state.command(functionCompletionCommand)
	if state.wordCompletion != nil || doc.Editor.String() != before || !doc.Editor.CompositionActive() {
		t.Fatal("function completion interrupted marked text")
	}
}

func TestMalformedAPIOverrideDoesNotCompleteOrShowParameters(t *testing.T) {
	state, driver := startDriver(t)
	dir := t.TempDir()
	state.preferencesPath = filepath.Join(dir, "settings.json")
	if err := os.Mkdir(filepath.Join(dir, "apis"), 0700); err != nil {
		t.Fatal(err)
	}
	const catalog = `corrupt header
<NotepadPlus><AutoComplete><KeyWord name="append" func="yes"><Overload><Param name="slice"/></Overload></KeyWord></AutoComplete></NotepadPlus>`
	if err := os.WriteFile(filepath.Join(dir, "apis", "go.xml"), []byte(catalog), 0600); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.languages[doc] = ".go"
	clickID(t, driver, editorID)
	if err := driver.Type("app"); err != nil {
		t.Fatal(err)
	}
	state.command(functionCompletionCommand)
	if doc.Editor.String() != "app" || state.wordCompletion != nil || !strings.Contains(state.notice, "invalid API catalog") {
		t.Errorf("malformed override changed completion: text=%q notice=%q", doc.Editor.String(), state.notice)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	doc.Editor.Insert("append(")
	state.command(parameterHintCommand)
	if doc.Editor.String() != "append(" || state.calltip != nil || !strings.Contains(state.notice, "invalid API catalog") {
		t.Fatalf("malformed override changed parameters: text=%q notice=%q", doc.Editor.String(), state.notice)
	}
}
