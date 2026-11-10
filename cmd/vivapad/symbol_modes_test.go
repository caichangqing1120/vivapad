package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func labelFor(state *model, character rune) (string, bool) {
	for _, item := range state.codeRepresentations() {
		if item.Character == character {
			return item.Label, true
		}
	}
	return "", false
}

func TestVisibleSourceRepresentationsUseDefaultBlob(t *testing.T) {
	state := newModel()
	state.command("Show Non-Printing Characters")
	for _, character := range []rune{'\u200b', '\x01', '\u2028'} {
		found := false
		for _, item := range state.codeRepresentations() {
			if item.Character == character {
				found = true
				if !item.Blob || item.Label == "" {
					t.Fatalf("U+%04X is not a visible Blob: %+v", character, item)
				}
			}
		}
		if !found {
			t.Fatalf("U+%04X missing from display", character)
		}
	}
	state.command("Show Control Characters & Unicode EOL")
	for _, item := range state.codeRepresentations() {
		if item.Character == '\x01' && (item.Blob || item.Label != "") {
			t.Fatalf("hidden control kept a Blob: %+v", item)
		}
	}
}

func TestNPCModesAndControlCouplingKeepSourceTextAndFlags(t *testing.T) {
	state := newModel()
	state.preferencesPath = filepath.Join(t.TempDir(), "VivaPad", "settings.json")
	doc := state.space.Current()
	const original = "A\u200bB\x01C\n"
	doc.Editor.Insert(original)
	doc.Editor.ClearHistory()
	state.command("Show Non-Printing Characters")
	if state.npcMode != "abbreviation" || state.npcIncludeControls {
		t.Fatal("source NPC mode defaults are wrong")
	}
	state.command("Non-Printing Mode: Codepoint")
	if npc, ok := labelFor(state, '\u200b'); !ok || npc != "U+200B" {
		t.Fatalf("NPC codepoint label = %q, %v", npc, ok)
	}
	if control, ok := labelFor(state, '\x01'); !ok || control != "SOH" {
		t.Fatalf("uncoupled control label = %q, %v", control, ok)
	}
	state.command("Include Control Characters in NPC Mode")
	if control, ok := labelFor(state, '\x01'); !ok || control != "U+0001" {
		t.Fatalf("coupled control label = %q, %v", control, ok)
	}
	state.command("Non-Printing Mode: Original")
	if npc, ok := labelFor(state, '\u200b'); !ok || npc != "\u200b" {
		t.Fatalf("original NPC label = %q, %v", npc, ok)
	}
	if control, ok := labelFor(state, '\x01'); !ok || control != "\x01" {
		t.Fatalf("original control label = %q, %v", control, ok)
	}
	state.command("Show All Characters")
	state.command("Show All Characters")
	if state.npcMode != "identity" || !state.npcIncludeControls || doc.Editor.String() != original || doc.Editor.CanUndo() {
		t.Fatal("Show All changed representation preferences or document data")
	}
	state.command("Show Control Characters & Unicode EOL")
	if control, ok := labelFor(state, '\x01'); !ok || control != "\x01" {
		t.Fatalf("re-enabled control did not retain identity mode: %q, %v", control, ok)
	}
	restored := newStartupModel(nil, filepath.Join(filepath.Dir(state.preferencesPath), "session.json"))
	if restored.npcMode != "identity" || !restored.npcIncludeControls {
		t.Fatalf("restored mode = %q, include=%v", restored.npcMode, restored.npcIncludeControls)
	}
}

func TestSymbolPreferencesModesFitCompactViewport(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(6))
	clickID(t, driver, menuItemScope.At(0))
	clickID(t, driver, preferencesSymbolsID)
	for index := range 3 {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == preferencesNPCModeScope.At(index) {
				found = true
				if hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("mode %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("NPC mode %d missing", index)
		}
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("NPC Preferences layout: %+v", problems)
	}
	clickID(t, driver, preferencesNPCModeScope.At(2))
	clickID(t, driver, preferencesNPCIncludeID)
	if state.npcMode != "codepoint" || !state.npcIncludeControls || state.space.Current().Dirty() {
		t.Fatalf("NPC Preferences changed text or not mode: mode=%q include=%v", state.npcMode, state.npcIncludeControls)
	}
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureNPCModesWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_NPC_MODE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_NPC_MODE_SHOTS to capture NPC modes")
	}
	state := newModel()
	state.preferencesPath = filepath.Join(t.TempDir(), "VivaPad", "settings.json")
	state.space.Current().Editor.Insert("A\u200b B\x01 C\u2028 D\n")
	state.command("Show Non-Printing Characters")
	state.command("Non-Printing Mode: Codepoint")
	state.command("Include Control Characters in NPC Mode")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 800}, {"compact", 640, 400},
	} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if err := session.Screenshot(filepath.Join(output, "npc-codepoint-editor-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), menuScope.At(6))
		clickID(t, session.Driver(), menuItemScope.At(0))
		clickID(t, session.Driver(), preferencesSymbolsID)
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s NPC mode preferences layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "npc-mode-preferences-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		if err := session.Driver().Key(ui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
	}
}
