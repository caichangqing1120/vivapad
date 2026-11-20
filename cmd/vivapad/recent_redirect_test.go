package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/recent"
)

func TestRecentCommandsKeepRecordedFilePath(t *testing.T) {
	for _, tc := range []struct {
		name          string
		restoreClosed bool
		redirectDir   bool
	}{
		{name: "recent menu and file symlink"},
		{name: "restore shortcut and directory symlink", restoreClosed: true, redirectDir: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			originalDir, otherDir := filepath.Join(root, "original"), filepath.Join(root, "other")
			for _, dir := range []string{originalDir, otherDir} {
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			path, other := filepath.Join(originalDir, "saved.txt"), filepath.Join(otherDir, "saved.txt")
			for file, data := range map[string][]byte{path: {0xd6, 0xd0, '\r', '\n'}, other: []byte("unrelated\n")} {
				if err := os.WriteFile(file, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			state := newStartupModel(nil, filepath.Join(root, "settings", "session.json"))
			if err := state.space.OpenWithEncoding(path, document.GB18030); err != nil {
				t.Fatal(err)
			}
			saved := state.space.Current()
			state.showInActivePane(saved)
			state.rememberRecent(saved)
			state.closeTab(state.space.Active)
			state.recentEntries = recent.Add(state.recentEntries, recent.Entry{Path: other})
			if err := recent.Save(state.recentPath, state.recentEntries); err != nil {
				t.Fatal(err)
			}
			current := state.space.Current()
			current.Editor.Insert("draft")
			selection := current.Editor.Selection()
			beforeRecent, beforeClosed := slices.Clone(state.recentEntries), slices.Clone(state.closedHistory)
			if len(beforeRecent) != 2 || len(beforeClosed) != 1 {
				t.Fatalf("invalid history fixture: recent=%v closed=%v", beforeRecent, beforeClosed)
			}
			read := func(path string) []byte {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			recentDisk, closedDisk := read(state.recentPath), read(state.closedPath)
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			open := func() {
				t.Helper()
				if tc.restoreClosed {
					if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 't'); err != nil || !handled {
						t.Fatalf("restore shortcut: handled=%v err=%v", handled, err)
					}
					return
				}
				clickID(t, driver, menuScope.At(0))
				clickID(t, driver, menuItemScope.At(10))
				clickID(t, driver, recentScope.At(1))
			}
			moved, target := path, other
			if tc.redirectDir {
				moved, target = originalDir, otherDir
			}
			parked := filepath.Join(root, "parked")
			if err := os.Rename(moved, parked); err != nil {
				t.Fatal(err)
			}
			workspaceFixtureLink(t, target, moved)
			open()
			if state.space.Current() != current || len(state.space.Documents) != 1 || !current.Dirty() || current.Editor.String() != "draft" || current.Editor.Selection() != selection {
				t.Errorf("redirect changed active buffer: path=%q notice=%q", state.space.Current().Path, state.notice)
			}
			if !slices.Equal(state.recentEntries, beforeRecent) || !slices.Equal(state.closedHistory, beforeClosed) ||
				!bytes.Equal(read(state.recentPath), recentDisk) || !bytes.Equal(read(state.closedPath), closedDisk) {
				t.Errorf("redirect changed history: recent=%v closed=%v", state.recentEntries, state.closedHistory)
			}
			if !strings.Contains(state.notice, "no longer resolves") {
				t.Fatalf("redirect was not reported: %q", state.notice)
			}
			if !tc.restoreClosed {
				if err := driver.Type("!"); err != nil || current.Editor.String() != "draft!" {
					t.Fatalf("first key after Recent Files refusal: draft=%q, err=%v", current.Editor.String(), err)
				}
				if err := driver.Key(gfx.KeyBackspace, 0); err != nil || current.Editor.String() != "draft" {
					t.Fatalf("editor did not receive backspace: draft=%q, err=%v", current.Editor.String(), err)
				}
			}
			if err := os.Remove(moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(parked, moved); err != nil {
				t.Fatal(err)
			}
			open()
			if got := state.space.Current(); got.Path != path || got.Editor.String() != "\u4e2d\n" || got.Encoding() != document.GB18030 || got.Dirty() {
				t.Fatalf("restored path did not reopen with its saved encoding: path=%q text=%q notice=%q", got.Path, got.Editor.String(), state.notice)
			}
			if len(state.closedHistory) != 0 || len(state.recentEntries) != 2 || state.recentEntries[0].Path != path ||
				current.Editor.String() != "draft" || !current.Dirty() {
				t.Fatal("successful retry did not update history or preserve the other draft")
			}
		})
	}
}
