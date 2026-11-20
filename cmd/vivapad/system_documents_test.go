package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemDocumentsOpenInActivePaneAndReuseDirtyTab(t *testing.T) {
	state, driver := startDriver(t)
	draft := state.space.Current()
	draft.Editor.Insert("left draft")
	state.toggleSplit()
	state.activatePane(1)
	path := filepath.Join(t.TempDir(), "new \u6587\u6863.txt")
	const original = "first\r\nsecond"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	state.queueSystemDocument(path)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	opened := state.space.Current()
	if state.activePane != 1 || state.paneDocument(0) != draft || state.paneDocument(1) != opened || opened.Editor.String() != "first\nsecond" {
		t.Fatal("system request did not open in the active pane while keeping the left draft")
	}
	opened.Editor.Insert("unsaved ")
	edited := opened.Editor.String()
	state.queueSystemDocument(path)
	state.queueSystemDocument(filepath.Join(filepath.Dir(path), "missing.txt"))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 2 || state.space.Current() != opened || !opened.Dirty() || opened.Editor.String() != edited || draft.Editor.String() != "left draft" {
		t.Fatal("reopening the same file or failing to open another replaced an existing edit")
	}
	if len(state.recentEntries) != 1 || state.recentEntries[0].Path != opened.Path || !strings.Contains(state.notice, "missing.txt") {
		t.Fatalf("recent entries = %v, status = %q", state.recentEntries, state.notice)
	}
	if contents, err := os.ReadFile(path); err != nil || string(contents) != original {
		t.Fatalf("source bytes = %q, error = %v", contents, err)
	}
}

func TestSystemDocumentsWaitForDirtyCloseDecision(t *testing.T) {
	state, driver := startDriver(t)
	draft := state.space.Current()
	draft.Editor.Insert("keep this draft")
	state.closeTab(0)
	path := filepath.Join(t.TempDir(), "new document.txt")
	if err := os.WriteFile(path, []byte("opened"), 0600); err != nil {
		t.Fatal(err)
	}
	state.queueSystemDocument(path)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.pendingClose != 0 || state.space.Current() != draft || len(state.space.Documents) != 1 {
		t.Fatal("incoming document interrupted the close decision")
	}
	state.cancelClose()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 2 || state.space.Current().Editor.String() != "opened" || draft.Editor.String() != "keep this draft" {
		t.Fatal("canceling Close did not preserve the draft and open the queued document")
	}
}

func TestSystemDocumentsReuseFileSpellingAlias(t *testing.T) {
	for _, test := range []struct{ name, original, alias string }{
		{"case", "Report.txt", "report.txt"},
		{"normalization", "caf\u00e9.txt", "cafe\u0301.txt"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path, alias := filepath.Join(root, test.original), filepath.Join(root, test.alias)
			const disk = "saved\r\ntext"
			if err := os.WriteFile(path, []byte(disk), 0600); err != nil {
				t.Fatal(err)
			}
			// Case-sensitive filesystems need a second name for the same file.
			if _, err := os.Stat(alias); os.IsNotExist(err) {
				if err := os.Link(path, alias); err != nil {
					t.Fatalf("create spelling alias: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			state.queueSystemDocument(path)
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			opened := state.space.Current()
			opened.Editor.Insert("draft ")
			before := opened.Editor.String()
			state.queueSystemDocument(alias)
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			if len(state.space.Documents) != 1 || state.space.Current() != opened || !opened.Dirty() || opened.Editor.String() != before {
				t.Fatalf("alias did not reuse the dirty tab: %d tabs, current %q, notice %q", len(state.space.Documents), state.space.Current().Path, state.notice)
			}
			if len(state.recentEntries) != 1 || state.recentEntries[0].Path != opened.Path {
				t.Fatalf("alias changed recent paths: %+v", state.recentEntries)
			}
			if read, err := os.ReadFile(path); err != nil || string(read) != disk {
				t.Fatalf("source bytes = %q, error = %v", read, err)
			}
		})
	}
}

func TestSystemDocumentsKeepDistinctFileSpellings(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "Report.txt"), filepath.Join(root, "report.txt")
	if err := os.WriteFile(first, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(second, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		t.Skip("test filesystem does not distinguish case")
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("second"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	state.queueSystemDocument(first)
	state.queueSystemDocument(second)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 2 || state.space.Current().Editor.String() != "second" {
		t.Fatal("distinct files with the same folded spelling were merged")
	}
}
