package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
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

func TestLoadNPPSharedAliasUsesActiveViewsSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		pane  int
		want  text.Range
		caret int
	}{
		{"main active", 0, text.Range{Start: 0, End: 1}, 0},
		{"sub active", 1, text.Range{Start: 2, End: 5}, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			const original = "\u732b\r\nabc"
			source := filepath.Join(root, "shared.txt")
			if err := os.WriteFile(source, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			sep := string(filepath.Separator)
			alias := root + sep + "." + sep + "shared.txt"
			xmlPath := filepath.Join(root, "session.xml")
			if err := session.SaveNPP(xmlPath, session.NPPState{
				Main:       []session.NPPFile{{Path: source, Encoding: -1, StartPos: 3, EndPos: 0}},
				Sub:        []session.NPPFile{{Path: alias, Encoding: -1, StartPos: 5, EndPos: 8}},
				ActiveView: tc.pane,
			}); err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			draft := state.space.Current()
			draft.Editor.Insert("keep my draft")
			draft.Editor.SetSelection(text.Range{Start: 4, End: 0})
			if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(0))
			clickID(t, driver, menuItemScope.At(13))
			clickID(t, driver, sessionScope.At(0))
			doc := state.space.Current()
			if !state.splitEnabled || state.activePane != tc.pane || len(state.space.Documents) != 2 ||
				state.paneDocs[0] != doc || state.paneDocs[1] != doc || doc == draft {
				t.Fatalf("session did not restore the shared file in the active pane: %q", state.notice)
			}
			if got := doc.Editor.Selection(); got != tc.want || doc.Editor.Caret() != tc.caret {
				t.Fatalf("active view selection = %+v, caret = %d; want %+v, %d", got, doc.Editor.Caret(), tc.want, tc.caret)
			}
			if doc.Dirty() || doc.Editor.CanUndo() || draft.Editor.String() != "keep my draft" ||
				draft.Editor.Selection() != (text.Range{Start: 0, End: 4}) || draft.Editor.Caret() != 0 || !draft.Dirty() {
				t.Fatal("import changed document content or edit history")
			}
			if data, err := os.ReadFile(source); err != nil || string(data) != original {
				t.Fatalf("import changed the source: %q, %v", data, err)
			}
		})
	}
}

func TestLoadNPPSharedAliasUsesSavedEncodingDuringPendingConversion(t *testing.T) {
	for _, tc := range []struct {
		name     string
		codePage int
		accept   bool
	}{
		{"matching decoder", 1252, true},
		{"matching pending save format", -1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			first, source := filepath.Join(root, "first.txt"), filepath.Join(root, "shared.txt")
			const original = "cat\r\ndog"
			for _, path := range []string{first, source} {
				if err := os.WriteFile(path, []byte(original), 0600); err != nil {
					t.Fatal(err)
				}
			}
			state, driver := startDriver(t)
			draft := state.space.Current()
			draft.Editor.Insert("keep my draft")
			if err := state.space.OpenWithEncoding(source, document.Windows1252); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			state.showInActivePane(doc)
			doc.SetEncoding(document.UTF8)
			before := text.Range{Start: 0, End: 3}
			doc.Editor.SetSelection(before)
			sep := string(filepath.Separator)
			alias := root + sep + "." + sep + "shared.txt"
			xmlPath := filepath.Join(root, "session.xml")
			if err := session.SaveNPP(xmlPath, session.NPPState{
				Main:       []session.NPPFile{{Path: first, Encoding: -1}, {Path: source, Encoding: tc.codePage}},
				Sub:        []session.NPPFile{{Path: alias, Encoding: tc.codePage, StartPos: 5, EndPos: 8}},
				ActiveMain: 1, ActiveView: 1,
			}); err != nil {
				t.Fatal(err)
			}
			if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(0))
			clickID(t, driver, menuItemScope.At(13))
			clickID(t, driver, sessionScope.At(0))
			if tc.accept {
				if !state.splitEnabled || state.activePane != 1 || state.paneDocs != [2]*document.Document{doc, doc} ||
					len(state.space.Documents) != 3 || doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
					t.Fatalf("matching decoded file did not restore as a clone: %q", state.notice)
				}
			} else if state.splitEnabled || state.space.Current() != doc || len(state.space.Documents) != 2 ||
				doc.Editor.Selection() != before || !strings.Contains(state.notice, "another encoding") {
				t.Fatalf("conflicting decoding changed the current workspace: %q", state.notice)
			}
			if doc.Encoding() != document.UTF8 || doc.SavedEncoding() != document.Windows1252 || !doc.Dirty() ||
				doc.Editor.String() != "cat\ndog" || doc.Editor.CanUndo() || draft.Editor.String() != "keep my draft" || !draft.Dirty() {
				t.Fatal("import changed text, Undo history, or the pending conversion")
			}
			if data, err := os.ReadFile(source); err != nil || string(data) != original {
				t.Fatalf("import changed source bytes: %q, %v", data, err)
			}
		})
	}
}

func TestLoadNPPRefusesAmbiguousOpenAliasesBeforeImporting(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "shared.txt")
	alias := filepath.Join(root, "SHARED.TXT")
	first := filepath.Join(root, "first.txt")
	const original = "saved\r\n"
	for _, path := range []string{source, first} {
		if err := os.WriteFile(path, []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(alias); os.IsNotExist(err) {
		t.Skip("temporary filesystem distinguishes these path spellings")
	} else if err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	for i, path := range []string{source, alias} {
		if err := state.space.Open(path); err != nil {
			t.Fatal(err)
		}
		opened := state.space.Current()
		state.showInActivePane(opened)
		opened.Editor.Insert([]string{"first draft ", "second draft "}[i])
		opened.Editor.SetSelection(text.Range{Start: i + 2, End: i})
	}
	if len(state.space.Documents) != 2 {
		t.Skip("workspace did not open two distinct tabs for the alias")
	}
	before := slices.Clone(state.space.Documents)
	active, panes := state.space.Current(), state.paneDocs
	recents := slices.Clone(state.recentEntries)
	xmlPath := filepath.Join(root, "session.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{
		Main:       []session.NPPFile{{Path: first, Encoding: -1}, {Path: alias, Encoding: -1, ReadOnly: true}},
		ActiveMain: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	if !strings.Contains(state.notice, "already open in multiple tabs") || !slices.Equal(state.space.Documents, before) ||
		state.space.Current() != active || state.paneDocs != panes || state.splitEnabled || !slices.Equal(state.recentEntries, recents) {
		t.Fatalf("ambiguous alias changed the workspace: tabs=%d notice=%q", len(state.space.Documents), state.notice)
	}
	for i, opened := range before {
		if opened.Editor.String() != []string{"first draft saved\n", "second draft saved\n"}[i] ||
			opened.Editor.Selection() != (text.Range{Start: i, End: i + 2}) || opened.Editor.Caret() != i ||
			!opened.Dirty() || !opened.Editor.CanUndo() || state.isReadOnly(opened) {
			t.Fatalf("ambiguous alias changed draft %d", i)
		}
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != original {
		t.Fatalf("session import changed the source: %q, %v", data, err)
	}
}
