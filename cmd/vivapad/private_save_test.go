package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
)

func TestDocumentSavesRefusePrivateDestinations(t *testing.T) {
	for _, tc := range []struct {
		name, command, reserved string
		alias                   bool
	}{
		{"save an open settings file", "save", "settings.json", false},
		{"save as settings", "save as", "settings.json", false},
		{"copy to recovery alias", "copy", "recovery.json", true},
		{"copy to saved commands alias", "copy", "run.json", true},
		{"export to named macros", "session", "named-macros.json", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			reserved := filepath.Join(root, tc.reserved)
			const privateBytes = `{"private":true}`
			if err := os.WriteFile(reserved, []byte(privateBytes), 0600); err != nil {
				t.Fatal(err)
			}
			source, target := filepath.Join(root, "editor.txt"), reserved
			if tc.command == "save" {
				source = reserved
			} else if err := os.WriteFile(source, []byte("editor\r\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.alias {
				target = filepath.Join(root, "alias.txt")
				if err := os.Link(reserved, target); err != nil {
					t.Skipf("hard links unavailable: %v", err)
				}
			}
			state := newModelWithFiles([]string{source})
			switch tc.reserved {
			case "settings.json":
				state.preferencesPath = reserved
			case "recovery.json":
				state.recoveryPath = reserved
			case "run.json":
				state.runPath = reserved
			case "named-macros.json":
				state.namedMacroPath = reserved
			}
			doc := state.space.Current()
			if tc.command != "session" {
				doc.Editor.Insert("draft ")
			}
			originalPath, value, dirty := doc.Path, doc.Editor.String(), doc.Dirty()
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			if tc.command == "save as" || tc.command == "copy" {
				if err := driver.QueueSaveFile(target, nil); err != nil {
					t.Fatal(err)
				}
			}
			switch tc.command {
			case "save":
				state.save(0, false)
			case "save as":
				state.save(0, true)
			case "copy":
				state.saveCopy(0)
			case "session":
				state.saveNPPSessionSelection(app.SaveFileSelection{Path: target})
			}
			if !strings.Contains(state.notice, "internal VivaPad data file") {
				t.Errorf("private destination was not refused: %q", state.notice)
			}
			for _, path := range []string{reserved, target} {
				if data, err := os.ReadFile(path); err != nil || string(data) != privateBytes {
					t.Errorf("private bytes changed at %s: %q, %v", path, data, err)
				}
			}
			if state.space.Current() != doc || doc.Path != originalPath || doc.Editor.String() != value || doc.Dirty() != dirty {
				t.Fatal("the attempted write changed the live document")
			}
		})
	}
}
