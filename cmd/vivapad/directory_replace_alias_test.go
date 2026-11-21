package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"vivapad/internal/document"
	"vivapad/internal/searchfiles"
)

func TestDirectoryReplaceRespectsCaseAliasesOfOpenTabs(t *testing.T) {
	testDirectoryReplacePathSpellings(t, "replace.txt", "REPLACE.txt")
}

func TestDirectoryReplaceRespectsUnicodeAliasesOfOpenTabs(t *testing.T) {
	testDirectoryReplacePathSpellings(t, "caf\u00e9.txt", "cafe\u0301.txt")
}

func testDirectoryReplacePathSpellings(t *testing.T, name, alternate string) {
	t.Helper()
	for _, tc := range []struct {
		name           string
		alias, isDirty bool
	}{
		{"dirty alias", true, true},
		{"clean alias", true, false},
		{"distinct files", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name)
			const original, replaced = "\xef\xbb\xbfcat\r\n", "\xef\xbb\xbfdog\r\n"
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 {
				t.Fatalf("test directory entries = %v: %v", entries, err)
			}
			diskName, otherName := entries[0].Name(), alternate
			if diskName == alternate {
				otherName = name
			}
			path = filepath.Join(root, diskName)
			opened := filepath.Join(root, otherName)
			_, err = os.Stat(opened)
			if tc.alias && os.IsNotExist(err) || !tc.alias && err == nil {
				t.Skip("test requires different filename equivalence rules")
			}
			if tc.alias && err != nil || !tc.alias && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if !tc.alias {
				if err := os.WriteFile(opened, []byte(original), 0600); err != nil {
					t.Fatal(err)
				}
			}
			state := newModelWithFiles([]string{opened})
			previous := state.space.Current()
			previous.Editor.MoveTo(2, false)
			if tc.isDirty {
				previous.Editor.Insert("local")
			}
			text, caret := previous.Editor.String(), previous.Editor.Caret()
			plan, err := searchfiles.PreviewReplace(t.Context(), root, "cat", "dog", document.SearchOptions{}, searchfiles.Options{Pattern: diskName})
			if err != nil || len(plan.Changes) != 1 {
				t.Fatalf("expected one planned file: %+v, %v", plan, err)
			}
			if previous.Path == "" || !tc.alias && previous.Path == plan.Changes[0].Path {
				t.Fatalf("unexpected open path %q for planned path %q", previous.Path, plan.Changes[0].Path)
			}
			if tc.alias && previous.Path == plan.Changes[0].Path {
				t.Log("platform resolved the alias to the planned path spelling")
			}
			state.replacePlan, state.replaceConfirmOpen = plan, true
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			clickID(t, driver, replaceConfirmID)
			for deadline := time.Now().Add(5 * time.Second); state.filesApplying && time.Now().Before(deadline); {
				if err := driver.Pump(); err != nil {
					t.Fatal(err)
				}
				time.Sleep(time.Millisecond)
			}
			if state.filesApplying {
				t.Fatal("replacement did not finish")
			}
			wantDisk := replaced
			if tc.alias && tc.isDirty {
				wantDisk = original
				if !state.replaceConfirmOpen || !strings.Contains(state.notice, "modified tabs") {
					t.Errorf("dirty open alias was not refused: %q", state.notice)
				}
			}
			if disk, err := os.ReadFile(path); err != nil || string(disk) != wantDisk {
				t.Errorf("target disk = %q, want %q: %v", disk, wantDisk, err)
			}
			current := state.space.Current()
			if tc.isDirty {
				if current != previous || !current.Dirty() || current.Editor.String() != text || current.Editor.Caret() != caret {
					t.Fatal("replacement changed the dirty editor or its selection")
				}
			} else if current == previous || current.Dirty() || current.Editor.String() != "dog\n" || current.Editor.Caret() != caret || state.replaceConfirmOpen {
				t.Fatal("clean open alias did not refresh after replacement")
			}
			if !tc.alias {
				if disk, err := os.ReadFile(opened); err != nil || string(disk) != original {
					t.Fatalf("distinct file was changed: %q, %v", disk, err)
				}
			}
		})
	}
}
