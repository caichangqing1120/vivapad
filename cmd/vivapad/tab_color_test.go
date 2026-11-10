package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/recovery"
	"vivapad/internal/session"
)

func TestTabColorsViewMenuAndCloneShareBufferWithoutEditing(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("draft")
	doc.Editor.ClearHistory()
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{doc, doc}
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	for color := 1; color <= 5; color++ {
		state.activePane = color % 2
		state.command(tabColorCommands[color-1])
		if state.tabColors[doc] != uint8(color) || doc.Editor.String() != "draft" || !doc.Dirty() || doc.Editor.CanUndo() {
			t.Fatalf("color %d changed buffer or missed clone", color)
		}
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(10))
	seen := make(map[string]bool)
	for step := 0; step < 30; step++ {
		for _, hit := range driver.Snapshot().Hits {
			if strings.HasPrefix(hit.Label, "Apply Color ") || hit.Label == "Remove Color" {
				seen[hit.Label] = true
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("color row outside compact viewport: %+v", hit.Bounds)
				}
			}
		}
		if len(seen) == 6 {
			break
		}
		clickID(t, driver, viewTabNextID)
	}
	if len(seen) != 6 {
		t.Fatalf("color menu rows reached: %+v", seen)
	}
	clickLabeledControl(t, driver, "Remove Color")
	if state.tabColors[doc] != 0 || doc.Editor.String() != "draft" {
		t.Fatal("Remove Color changed the document")
	}
}

func TestTabColorsPrivateSessionAndDirtyRecovery(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "saved.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, sessionPath)
	state.command("Apply Color 3")
	newConfig(state).OnCloseRequest(func() {})
	stored, err := session.Load(sessionPath)
	if err != nil || stored.TabColors[state.space.Current().Path] != 3 {
		t.Fatalf("saved tab color = %+v, %v", stored.TabColors, err)
	}
	restored := newStartupModel(nil, sessionPath)
	if restored.tabColors[restored.space.Current()] != 3 {
		t.Fatal("session restart lost saved color")
	}
	untitled := newModel()
	unsaved := untitled.space.Current()
	unsaved.Editor.Insert("draft")
	untitled.command("Apply Color 5")
	backup := untitled.currentRecovery()
	if len(backup.Tabs) != 1 || backup.Tabs[0].TabColor != 5 {
		t.Fatalf("dirty recovery color = %+v", backup)
	}
	if err := recovery.Save(filepath.Join(root, "settings", "recovery.json"), backup); err != nil {
		t.Fatal(err)
	}
	crashed := newStartupModel(nil, filepath.Join(root, "settings", "fresh-session.json"))
	if crashed.tabColors[crashed.space.Current()] != 5 || crashed.space.Current().Editor.String() != "draft" {
		t.Fatalf("recovered tab color = %v, text %q", crashed.tabColors, crashed.space.Current().Editor.String())
	}
}

func TestNPPColorsRejectConflictingCloneBeforeMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "shared.txt")
	if err := os.WriteFile(path, []byte("shared"), 0600); err != nil {
		t.Fatal(err)
	}
	xmlPath := filepath.Join(root, "session.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{
		Main: []session.NPPFile{{Path: path, Encoding: -1, Color: 1}},
		Sub:  []session.NPPFile{{Path: path, Encoding: -1, Color: 2}}, ActiveView: 1,
	}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	before := state.space.Current()
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	state.loadNPPSession()
	if state.space.Current() != before || len(state.space.Documents) != 1 || len(state.tabColors) != 0 || !strings.Contains(state.notice, "conflicting") {
		t.Fatalf("conflicting clone mutated workspace: %q", state.notice)
	}
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
}

func TestNPPColorsExportAndImport(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "colored.txt")
	if err := os.WriteFile(path, []byte("clean"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.command("Apply Color 4")
	stored, err := state.nppSessionState()
	if err != nil || len(stored.Main) != 1 || stored.Main[0].Color != 4 {
		t.Fatalf("NPP export color = %+v, %v", stored, err)
	}
	xmlPath := filepath.Join(root, "session.xml")
	if err := session.SaveNPP(xmlPath, stored); err != nil {
		t.Fatal(err)
	}
	loaded, driver := startDriver(t)
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	loaded.loadNPPSession()
	if len(loaded.space.Documents) != 1 || loaded.tabColors[loaded.space.Current()] != 4 {
		t.Fatalf("NPP import color = %v, notice %q", loaded.tabColors, loaded.notice)
	}
}

func TestTabColorsSurviveReloadAndAppearInDocumentList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "colored.txt")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.command("Apply Color 2")
	state.command("Document List")
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	seen := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Open colored.txt" && strings.Contains(hit.Hint, "Apply Color 2") {
			seen = true
		}
	}
	if !seen {
		t.Fatal("Document List did not expose the tab color")
	}
	if err := os.WriteFile(path, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	state.command("Reload from Disk")
	if state.tabColors[state.space.Current()] != 2 || state.space.Current().Editor.String() != "second" {
		t.Fatal("reload lost the tab color or text")
	}
}

func TestCaptureTabColorsWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_TAB_COLOR_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_TAB_COLOR_SHOTS to capture colored tabs")
	}
	state := newModel()
	for index := 1; index <= 5; index++ {
		doc := state.space.Current()
		if index > 1 {
			doc = state.space.NewDocument()
		}
		doc.Name = "color" + string(rune('0'+index)) + ".txt"
		state.showInActivePane(doc)
		state.command(tabColorCommands[index-1])
	}
	state.command("Document List")
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
		if size.name == "compact" {
			clickID(t, session.Driver(), menuScope.At(3))
			clickID(t, session.Driver(), menuItemScope.At(10))
			for index := 0; index < len(viewTabCommands)-6; index++ {
				if state.viewTabOffset >= len(viewTabCommands)-state.viewTabVisible {
					break
				}
				clickID(t, session.Driver(), viewTabNextID)
			}
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "tab-colors-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}
