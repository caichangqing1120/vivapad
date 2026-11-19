package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/session"
)

func TestSessionExportProtectsExistingDestinationsByFileIdentity(t *testing.T) {
	for _, item := range []struct {
		name    string
		private bool
		link    bool
		refusal string
	}{
		{"open tab hard link", false, true, "already open in a tab"},
		{"private data hard link", true, true, "internal VivaPad data file"},
		{"open tab case alias", false, false, "already open in a tab"},
		{"private data case alias", true, false, "internal VivaPad data file"},
	} {
		t.Run(item.name, func(t *testing.T) {
			root := t.TempDir()
			openPath, protected := filepath.Join(root, "Open.txt"), filepath.Join(root, "Open.txt")
			alias := filepath.Join(root, "open.txt")
			if item.private {
				protected, alias = filepath.Join(root, "Settings.json"), filepath.Join(root, "settings.json")
			}
			if err := os.WriteFile(openPath, []byte("document\r\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if item.private {
				if err := os.WriteFile(protected, []byte(`{"private":true}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if item.link {
				alias = filepath.Join(root, "hardlink.xml")
				if err := os.Link(protected, alias); err != nil {
					t.Skipf("hard links unavailable: %v", err)
				}
			} else if _, err := os.Stat(alias); os.IsNotExist(err) {
				t.Skip("filesystem distinguishes filename case")
			} else if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(protected)
			if err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{openPath})
			if item.private {
				state.preferencesPath = protected
			}
			doc := state.space.Current()
			state.saveNPPSessionSelection(app.SaveFileSelection{Path: alias})
			if !strings.Contains(state.notice, item.refusal) {
				t.Errorf("alias export was not refused: %q", state.notice)
			}
			for _, path := range []string{protected, alias} {
				if data, err := os.ReadFile(path); err != nil || string(data) != string(before) {
					t.Errorf("protected bytes changed at %s: %q, %v", path, data, err)
				}
			}
			if state.space.Current() != doc || doc.Dirty() || doc.Editor.String() != "document\n" {
				t.Fatal("export touched the live document")
			}
			independent := filepath.Join(root, "independent.xml")
			if err := os.WriteFile(independent, before, 0600); err != nil {
				t.Fatal(err)
			}
			state.saveNPPSessionSelection(app.SaveFileSelection{Path: independent})
			if stored, err := session.LoadNPP(independent); err != nil || len(stored.Main) != 1 || stored.Main[0].Path != doc.Path {
				t.Fatalf("independent destination was not saved: %+v, %v", stored, err)
			}
		})
	}
}

func TestSessionExportAllowsDistinctNamesOnCaseSensitiveFilesystems(t *testing.T) {
	root := t.TempDir()
	openPath, output := filepath.Join(root, "Report.txt"), filepath.Join(root, "report.txt")
	const original = "document\r\n"
	if err := os.WriteFile(openPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); err == nil {
		t.Skip("filesystem aliases filename case")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{openPath})
	state.saveNPPSessionSelection(app.SaveFileSelection{Path: output})
	if stored, err := session.LoadNPP(output); err != nil || len(stored.Main) != 1 || stored.Main[0].Path != state.space.Current().Path {
		t.Fatalf("same-name destination is independent: %+v, %v", stored, err)
	}
	if got, err := os.ReadFile(openPath); err != nil || string(got) != original {
		t.Fatalf("export touched the open file: %q, %v", got, err)
	}
}
