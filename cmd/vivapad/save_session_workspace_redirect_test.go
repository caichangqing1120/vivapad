package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/session"
)

func TestSessionExportRefusesRedirectedWorkspaceOnlyWhenIncluded(t *testing.T) {
	for _, mode := range []string{"fallback", "native"} {
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			root, outside := filepath.Join(base, "workspace"), filepath.Join(base, "other")
			for _, path := range []string{root, outside} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			file, output := filepath.Join(base, "saved.txt"), filepath.Join(base, "session.xml")
			if err := os.WriteFile(file, []byte("saved\r\n"), 0600); err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{file})
			if err := state.addFolderRoot(root); err != nil {
				t.Fatal(err)
			}
			doc, recordedRoot := state.space.Current(), state.folderRoots[0]
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			redirect := func() {
				if err := os.Rename(root, filepath.Join(base, "parked")); err != nil {
					t.Fatal(err)
				}
				workspaceFixtureLink(t, outside, root)
			}
			if mode == "fallback" {
				state.command("Save Session...")
				state.invalidator.Invalidate()
				if err := driver.Pump(); err != nil {
					t.Fatal(err)
				}
				redirect()
				if err := driver.QueueSaveFile(output, nil); err != nil {
					t.Fatal(err)
				}
				clickID(t, driver, saveSessionConfirmID)
				if !state.saveSessionOptionsOpen || !strings.Contains(state.saveSessionError, "folder unavailable") {
					t.Fatalf("redirected workspace advanced to the save picker: %q", state.saveSessionError)
				}
			} else {
				state.saveSessionPicker = func(_ app.SaveFileOptions, checkbox app.SaveFileCheckbox) (app.SaveFileSelection, error) {
					if !checkbox.Checked {
						t.Fatal("visible workspace checkbox should start checked")
					}
					redirect()
					return app.SaveFileSelection{Path: output, Checked: true}, nil
				}
				state.command("Save Session...")
				if !strings.Contains(state.notice, "folder unavailable") {
					t.Fatalf("native session accepted a root redirected during the picker: %q", state.notice)
				}
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("checked export wrote a file: %v", err)
			}
			if mode == "fallback" {
				clickID(t, driver, saveSessionWorkspaceID)
				clickID(t, driver, saveSessionConfirmID)
			} else {
				state.saveSessionPicker = app.SaveFileWithCheckbox
				if err := driver.QueueSaveFileWithCheckbox(output, false, nil); err != nil {
					t.Fatal(err)
				}
				state.command("Save Session...")
			}
			stored, err := session.LoadNPP(output)
			if err != nil || stored.FileBrowserIncluded || len(stored.Main) != 1 || stored.Main[0].Path != doc.Path {
				t.Fatalf("unchecked session = %+v, %v", stored, err)
			}
			if state.space.Current() != doc || doc.Dirty() || !state.folderVisible || len(state.folderRoots) != 1 || state.folderRoots[0] != recordedRoot {
				t.Fatal("export changed the workspace or active document")
			}
		})
	}
}
