package main

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/preferences"
)

func TestInternetSearchCompactMenuAndSavedEngine(t *testing.T) {
	state := newStartupModel(nil, filepath.Join(t.TempDir(), "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	const source = "猫 & dog"
	doc.Editor.Insert(source)
	selection := text.Range{Start: 0, End: doc.Editor.RuneCount()}
	doc.Editor.SetSelection(selection)
	open := func(index int) {
		clickID(t, driver, menuScope.At(1))
		revealEditMenuItem(t, driver, 24)
		clickID(t, driver, menuItemScope.At(24))
		clickID(t, driver, onSelectionScope.At(index))
	}
	open(5)
	clickID(t, driver, webSearchEngineScope.At(1))
	clickID(t, driver, webSearchSaveID)
	requireEditorCommandFocus(t, driver, editorID)
	if _, ok := driver.LastOpenedURL(); ok {
		t.Fatal("choosing an engine opened a browser")
	}
	stored, err := preferences.Load(state.preferencesPath)
	if err != nil || stored.WebSearch.Engine != "google" {
		t.Fatalf("saved preference = %+v, %v", stored, err)
	}
	open(5)
	clickID(t, driver, webSearchEngineScope.At(2))
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	open(4)
	opened, ok := driver.LastOpenedURL()
	parsed, err := url.Parse(opened)
	if !ok || err != nil || parsed.Host != "www.google.com" || parsed.Query().Get("q") != source {
		t.Fatalf("browser target %q: %v", opened, err)
	}
	requireEditorCommandFocus(t, driver, editorID)
	if doc.Editor.String() != source || doc.Editor.Selection() != selection {
		t.Fatal("search changed the editor")
	}
}

func TestInternetSearchUsesActivePaneAndRejectsMultiline(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	const word = "va\u0308lue_42"
	right.Editor.Insert(word + "\nsecond")
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 1
	state.space.Select(state.documentIndex(right))
	right.Editor.SetSelection(text.Range{Start: 0, End: right.Editor.RuneCount()})
	state.command(webSearchCommand)
	if _, ok := driver.LastOpenedURL(); ok || !strings.Contains(state.notice, "single") {
		t.Fatalf("multiline search was not refused: %q", state.notice)
	}
	right.Editor.MoveTo(2, false)
	state.command(webSearchCommand)
	target, ok := driver.LastOpenedURL()
	parsed, err := url.Parse(target)
	if !ok || err != nil || parsed.Host != "duckduckgo.com" || parsed.Query().Get("q") != word {
		t.Fatalf("active-pane search = %q, %v", target, err)
	}
}

func TestInternetSearchRefusesNativeMultipleSelections(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const source = "alpha beta"
	doc.Editor.Insert(source)
	doc.Editor.SetSelections([]text.Range{{Start: 0, End: 5}, {Start: 6, End: 10}}, 0)
	if doc.Editor.SelectionCount() != 2 {
		t.Fatal("native selections were not established")
	}
	state.command(webSearchCommand)
	if target, ok := driver.LastOpenedURL(); ok || !strings.Contains(state.notice, "single text range") {
		t.Fatalf("multiple selections: opened=%t, target=%q, notice=%q", ok, target, state.notice)
	}
	if doc.Editor.String() != source || doc.Editor.SelectionCount() != 2 {
		t.Fatal("refusing the search changed the editor")
	}
}

func TestInternetSearchRefusesLiveCompositionWithoutCancelingIt(t *testing.T) {
	for _, tentative := range []bool{false, true} {
		name := "legacy"
		if tentative {
			name = "tentative"
		}
		t.Run(name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("original")
			doc.Editor.SetSelection(text.Range{Start: 0, End: 8})
			if tentative {
				if !doc.Editor.TentativeCompositionEnabled() && !doc.Editor.EnableTentativeComposition() {
					t.Fatal("tentative composition was not enabled")
				}
				input := state.macroTextInput(doc)
				if _, err := input.OnEvent(app.TextEvent{Kind: app.TextSetMarked, CompositionPhase: text.CompositionUpdate,
					Text: "preview", MarkedSelection: app.TextRange{Start: 7, End: 7}}); err != nil {
					t.Fatal(err)
				}
			} else {
				doc.Editor.DisableTentativeComposition()
				doc.Editor.SetMarkedText("preview", text.Range{Start: 7, End: 7})
			}
			if !doc.Editor.CompositionActive() || doc.Editor.String() != "preview" {
				t.Fatal("composition fixture was not established")
			}
			selection := doc.Editor.Selection()
			state.command(webSearchCommand)
			if target, ok := driver.LastOpenedURL(); ok || !strings.Contains(state.notice, "composition") {
				t.Fatalf("live composition: opened=%t, target=%q, notice=%q", ok, target, state.notice)
			}
			if !doc.Editor.CompositionActive() || doc.Editor.String() != "preview" || doc.Editor.Selection() != selection {
				t.Fatal("refusing the search changed the composition")
			}
		})
	}
}

func TestInternetSearchCustomSettingsSaveFailureDoesNotChangeEngine(t *testing.T) {
	state, driver := startDriver(t)
	state.command(webSearchSettingsCommand)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, webSearchEngineScope.At(4))
	clickID(t, driver, webSearchTemplateID)
	if err := driver.Type("https://example.test/?q=$(CURRENT_WORD)"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, webSearchSaveID)
	if !state.webSearchOpen || state.webSearchError == "" || state.webSearch.EngineID() != "duckduckgo" {
		t.Fatalf("save failure committed settings: %+v, %q", state.webSearch, state.webSearchError)
	}
	state.preferencesPath = filepath.Join(t.TempDir(), "settings.json")
	clickID(t, driver, webSearchSaveID)
	stored, err := preferences.Load(state.preferencesPath)
	if err != nil || stored.WebSearch.Engine != "custom" || stored.WebSearch.Template != "https://example.test/?q=$(CURRENT_WORD)" {
		t.Fatalf("custom engine not restored: %+v, %v", stored.WebSearch, err)
	}
	if _, ok := driver.LastOpenedURL(); ok {
		t.Fatal("saving a custom template opened a browser")
	}
}
