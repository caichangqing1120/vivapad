package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/session"
)

func TestLoadNPPCanonicalizesFileIdentityPerPane(t *testing.T) {
	for _, tc := range []struct {
		name, sharedName, aliasName string
		split, distinct             bool
	}{
		{"duplicate within pane", "shared.txt", "", false, false},
		{"clone across panes", "shared.txt", "", true, false},
		{"case alias within pane", "shared.txt", "SHARED.TXT", false, false},
		{"case alias across panes", "shared.txt", "SHARED.TXT", true, false},
		{"distinct files with different case", "shared.txt", "SHARED.TXT", false, true},
		{"Unicode alias within pane", "caf\u00e9.txt", "cafe\u0301.txt", false, false},
		{"Unicode alias across panes", "caf\u00e9.txt", "cafe\u0301.txt", true, false},
		{"distinct files with different Unicode forms", "caf\u00e9.txt", "cafe\u0301.txt", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			first, shared := filepath.Join(root, "first.txt"), filepath.Join(root, tc.sharedName)
			for _, path := range []string{first, shared} {
				if err := os.WriteFile(path, []byte("saved\r\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			sep := string(filepath.Separator)
			alias := root + sep + "." + sep + tc.sharedName
			if tc.aliasName != "" {
				alias = filepath.Join(root, tc.aliasName)
			}
			if tc.distinct {
				file, err := os.OpenFile(alias, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if os.IsExist(err) {
					t.Skip("temporary filesystem treats these names as aliases")
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			} else if tc.aliasName != "" {
				info, err := os.Stat(alias)
				if os.IsNotExist(err) {
					t.Skip("temporary filesystem distinguishes these names")
				}
				if err != nil {
					t.Fatal(err)
				}
				original, err := os.Stat(shared)
				if err != nil || !os.SameFile(original, info) {
					t.Fatalf("alias does not name the fixture: %v", err)
				}
			}
			entry := session.NPPFile{Path: alias, Encoding: -1}
			stored := session.NPPState{Main: []session.NPPFile{
				{Path: first, Encoding: -1}, {Path: shared, Encoding: -1},
			}, ActiveMain: 1}
			if tc.split {
				stored.Sub = []session.NPPFile{entry}
				stored.ActiveView = 1
			} else {
				stored.Main = append(stored.Main, entry)
				stored.ActiveMain = 2
			}
			xmlPath := filepath.Join(root, "session.xml")
			if err := session.SaveNPP(xmlPath, stored); err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			draft := state.space.Current()
			draft.Editor.Insert("keep my draft")
			selection := text.Range{Start: 0, End: 4}
			draft.Editor.SetSelection(selection)
			if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(0))
			clickID(t, driver, menuItemScope.At(13))
			clickID(t, driver, sessionScope.At(0))
			if draft.Editor.String() != "keep my draft" || draft.Editor.Selection() != selection || !draft.Dirty() {
				t.Fatal("session import changed the existing draft")
			}
			if tc.distinct {
				if len(state.space.Documents) != 4 || state.space.Current().Name != tc.aliasName || state.splitEnabled {
					t.Fatalf("distinct names did not open separately: %d tabs, %q", len(state.space.Documents), state.notice)
				}
				return
			}
			if !tc.split {
				if len(state.space.Documents) != 1 || state.space.Current() != draft || state.splitEnabled || !strings.Contains(state.notice, "duplicate file") {
					t.Fatalf("same-pane alias was not refused before import: %d tabs, %q", len(state.space.Documents), state.notice)
				}
				return
			}
			if len(state.space.Documents) != 3 || !state.splitEnabled || state.activePane != 1 || state.paneDocs[0] != state.paneDocs[1] || state.space.Current().Name != tc.sharedName {
				t.Fatalf("cross-pane alias did not restore a shared document: %d tabs, %q", len(state.space.Documents), state.notice)
			}
		})
	}
}
