package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/recent"
	"vivapad/internal/searchfiles"
)

func TestFileOpenReleasesReplacedUntitledEditor(t *testing.T) {
	for _, route := range []string{"picker", "encoding picker", "drop", "system", "recent", "restore closed", "folder", "find result"} {
		t.Run(route, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "note.txt")
			if err := os.WriteFile(path, []byte("note"), 0600); err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			blank := state.space.Current()
			if state.editors[blank] == nil {
				t.Fatal("blank editor was not painted")
			}
			switch route {
			case "picker", "encoding picker":
				if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
					t.Fatal(err)
				}
				clickID(t, driver, menuScope.At(0))
				if route == "encoding picker" {
					clickMenuLabel(t, driver, "Open as GB18030...")
				} else {
					clickMenuLabel(t, driver, "Open...")
				}
			case "drop":
				if accepted, err := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, X: 100, Y: 150, Paths: []string{path}}); err != nil || !accepted {
					t.Fatalf("drop: accepted=%v, err=%v", accepted, err)
				}
			case "system":
				state.queueSystemDocument(path)
			case "recent":
				state.recentEntries = []recent.Entry{{Path: path}}
				state.openRecent(0)
			case "restore closed":
				state.closedHistory = []recent.Entry{{Path: path}}
				state.restoreClosed()
			case "folder":
				if !state.activateFolderRow(folderRow{path: path, name: "note.txt"}) {
					t.Fatal(state.notice)
				}
			case "find result":
				state.fileResults.Matches = []searchfiles.Match{{Path: path, Source: "note", Start: 0, End: 4, Text: "note"}}
				if !state.openFileResult(0) {
					t.Fatal(state.notice)
				}
			}
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			current := state.space.Current()
			if len(state.space.Documents) != 1 || current.Path != path || state.paneDocs[0] != current {
				t.Fatal("open no longer replaces the sole untitled tab")
			}
			if _, retained := state.paneOwner[blank]; retained || state.editors[blank] != nil || state.secondEditors[blank] != nil || len(state.closedHistory) > 0 {
				t.Fatal("replaced tab retained its editor, pane ownership, or closed-file entry")
			}
		})
	}
}

func TestFileOpenKeepsPeerUntitledTabEditableAndSaveable(t *testing.T) {
	root := t.TempDir()
	path, saved := filepath.Join(root, "note.txt"), filepath.Join(root, "draft.txt")
	if err := os.WriteFile(path, []byte("note\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	blank := state.space.Current()
	state.toggleSplit()
	state.activatePane(1)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if state.openDocumentPaths([]string{filepath.Join(root, "missing.txt")}) || state.paneDocs[0] != blank || state.paneDocs[1] != blank || state.space.Current() != blank {
		t.Fatal("failed open changed the shared untitled tab")
	}
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickMenuLabel(t, driver, "Open...")
	opened := state.space.Current()
	if len(state.space.Documents) != 2 || state.paneDocs[0] != blank || state.paneDocs[1] != opened || state.paneOwner[blank] != 0 || state.paneOwner[opened] != 1 || opened.Editor.String() != "note\n" {
		t.Fatal("opening a file orphaned or replaced the peer pane's tab")
	}
	clickID(t, driver, editorID)
	if err := driver.Type("draft"); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueSaveFile(saved, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveID)
	if data, err := os.ReadFile(saved); err != nil || string(data) != "draft" {
		t.Fatalf("Save did not write the peer pane's draft: %q, %v", data, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "note\r\n" {
		t.Fatalf("Save changed the unrelated source: %q, %v", data, err)
	}
}
