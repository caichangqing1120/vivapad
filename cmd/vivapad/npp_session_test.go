package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/session"
)

func TestSaveNPPSessionCapturesSplitViewsWithoutChangingTabs(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.go"), filepath.Join(root, "second.txt")
	if err := os.WriteFile(first, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte{0xd6, 0xd0}, 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{first})
	left := state.space.Current()
	if err := state.space.OpenWithEncoding(second, document.GB18030); err != nil {
		t.Fatal(err)
	}
	right := state.space.Current()
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(1)
	right.Editor.SetSelection(text.Range{Start: 0, End: 1})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	path := filepath.Join(root, "saved-session.xml")
	state.space.Select(0)
	state.command("Toggle Bookmark")
	state.command("Read-Only on Current Document")
	state.space.Select(1)
	state.command("Toggle Bookmark")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(1))
	clickID(t, driver, saveSessionConfirmID)
	stored, err := session.LoadNPP(path)
	if err != nil || len(stored.Main) != 1 || len(stored.Sub) != 1 || stored.ActiveView != 1 || stored.Sub[0].Encoding != 54936 || stored.Sub[0].EndPos != 2 || len(stored.Main[0].Marks) != 1 || stored.Main[0].Marks[0] != 0 || len(stored.Sub[0].Marks) != 1 || stored.Sub[0].Marks[0] != 0 || !stored.Main[0].ReadOnly {
		t.Fatalf("saved split session = %+v, %v", stored, err)
	}
	if state.space.Current() != right || state.paneDocs[0] != left || right.Dirty() {
		t.Fatal("export mutated the active tabs")
	}
}

func TestLoadNPPAddsViewsWithoutDiscardingDirtyBuffer(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.go"), filepath.Join(root, "second.txt")
	if err := os.WriteFile(first, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte{0xd6, 0xd0, '\r', '\n'}, 0600); err != nil {
		t.Fatal(err)
	}
	paths := []string{first, second}
	for index, path := range paths {
		paths[index], _ = filepath.EvalSymlinks(path)
	}
	xmlPath := filepath.Join(root, "import.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{
		Main:       []session.NPPFile{{Path: paths[0], Encoding: -1, Language: "go", Marks: []int{0}, ReadOnly: true}},
		Sub:        []session.NPPFile{{Path: paths[1], Encoding: 54936, StartPos: 0, EndPos: 3}},
		ActiveView: 1,
	}); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, filepath.Join(root, "auto", "session.json"))
	dirty := state.space.Current()
	dirty.Editor.Insert("draft")
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	if len(state.space.Documents) != 3 || state.space.Documents[0] != dirty || !dirty.Dirty() || !state.splitEnabled || state.activePane != 1 {
		t.Fatalf("loaded session lost dirty tab or view: docs=%+v split=%v pane=%d notice=%q", state.space.Documents, state.splitEnabled, state.activePane, state.notice)
	}
	if left, right := state.paneDocs[0], state.paneDocs[1]; left.Path != paths[0] || right.Path != paths[1] || right.Encoding() != document.GB18030 || right.Editor.String() != "中\n" || right.Editor.Selection() != (text.Range{Start: 0, End: 1}) {
		t.Fatalf("imported panes = %+v / %+v", left, right)
	}
	if marks := state.bookmarkLines(state.paneDocs[0]); len(marks) != 1 || marks[0] != 0 {
		t.Fatalf("imported marks = %v", marks)
	}
	if !state.isReadOnly(state.paneDocs[0]) || state.isReadOnly(state.paneDocs[1]) {
		t.Fatal("imported read-only flags did not follow their documents")
	}
}

func TestLoadNPPPreflightFailureDoesNotOpenEarlierFiles(t *testing.T) {
	root := t.TempDir()
	valid, missing := filepath.Join(root, "valid.txt"), filepath.Join(root, "missing.txt")
	if err := os.WriteFile(valid, []byte("valid"), 0600); err != nil {
		t.Fatal(err)
	}
	xmlPath := filepath.Join(root, "invalid-session.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{Main: []session.NPPFile{{Path: valid, Encoding: -1}, {Path: missing, Encoding: -1}}}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	dirty := state.space.Current()
	dirty.Editor.Insert("keep")
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	if len(state.space.Documents) != 1 || state.space.Current() != dirty || dirty.Editor.String() != "keep" || !strings.Contains(state.notice, "missing.txt") {
		t.Fatalf("failed NPP import changed workspace: %+v notice=%q", state.space.Documents, state.notice)
	}
}

func TestLoadNPPRejectsOutOfRangeBookmarksBeforeOpeningTabs(t *testing.T) {
	root := t.TempDir()
	valid, invalid := filepath.Join(root, "valid.txt"), filepath.Join(root, "short.txt")
	for _, path := range []string{valid, invalid} {
		if err := os.WriteFile(path, []byte("only one line"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	xmlPath := filepath.Join(root, "marks.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{Main: []session.NPPFile{
		{Path: valid, Encoding: -1}, {Path: invalid, Encoding: -1, Marks: []int{1}},
	}}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	dirty := state.space.Current()
	dirty.Editor.Insert("keep")
	state.command("Toggle Bookmark")
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	if len(state.space.Documents) != 1 || state.space.Current() != dirty || dirty.Editor.String() != "keep" || len(state.bookmarkLines(dirty)) != 1 || !strings.Contains(state.notice, "out of range") {
		t.Fatalf("invalid bookmark changed workspace: docs=%+v, notice=%q", state.space.Documents, state.notice)
	}
}

func TestLoadNPPRejectsConflictingReadOnlyCloneBeforeChangingWorkspace(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "shared.txt")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	xmlPath := filepath.Join(root, "conflict.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{
		Main:       []session.NPPFile{{Path: path, Encoding: -1, ReadOnly: true}},
		Sub:        []session.NPPFile{{Path: path, Encoding: -1}},
		ActiveView: 1,
	}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	dirty := state.space.Current()
	dirty.Editor.Insert("keep")
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	if len(state.space.Documents) != 1 || state.space.Current() != dirty || !dirty.Dirty() || !strings.Contains(state.notice, "conflicting") {
		t.Fatalf("conflicting clone partially imported: docs=%+v notice=%q", state.space.Documents, state.notice)
	}
}

func TestSaveNPPSessionRefusesDirtyOrOpenDestination(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "current.xml")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.space.Current().Editor.Insert("dirty")
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(1))
	if !strings.Contains(state.notice, "Save modified tabs") {
		t.Fatalf("dirty export was not refused: %q", state.notice)
	}
	state.space.Current().Editor.Undo()
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(1))
	clickID(t, driver, saveSessionConfirmID)
	if data, err := os.ReadFile(path); err != nil || string(data) != "original" || !strings.Contains(state.notice, "already open") {
		t.Fatalf("session export overwrote an open tab: %q, %v, notice=%q", data, err, state.notice)
	}
}

func TestNPPPositionConversionAccountsForCRLFAndUnicode(t *testing.T) {
	doc := document.New("new 1")
	doc.Editor.Insert("猫\nabc")
	doc.SetLineEnding(document.CRLF)
	if got := nppBytePosition(doc, 2); got != 5 {
		t.Fatalf("NPP byte position after 猫 CRLF = %d, want 5", got)
	}
	if got := nppSelectionAt(doc, 5, 5).Start; got != 2 {
		t.Fatalf("rune position after 猫 CRLF = %d, want 2", got)
	}
}

func TestNPPLanguageShortAliasesRemainImportable(t *testing.T) {
	for alias, mode := range map[string]string{"js": ".js", "ts": ".ts", "py": ".py", "md": ".md", "txt": ""} {
		if got, err := nppExtension(alias); err != nil || got != mode {
			t.Fatalf("NPP language %q = %q, %v; want %q", alias, got, err, mode)
		}
	}
}

func TestLoadNPPSessionWithOnlySubViewKeepsRightOwnership(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sub.txt")
	if err := os.WriteFile(path, []byte("right"), 0600); err != nil {
		t.Fatal(err)
	}
	xmlPath := filepath.Join(root, "sub-only.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{Sub: []session.NPPFile{{Path: path, Encoding: -1}}, ActiveView: 1}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	doc := state.space.Current()
	if len(state.space.Documents) != 1 || !state.splitEnabled || state.activePane != 1 || state.paneDocs[1] != doc || state.paneOwner[doc] != 1 {
		t.Fatalf("sub-only view = docs:%+v panes:%+v owner:%d notice:%q", state.space.Documents, state.paneDocs, state.paneOwner[doc], state.notice)
	}
}

func TestLoadNPPCrlfUnicodeSelectionUsesSourceBytePositions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "unicode.txt")
	if err := os.WriteFile(path, []byte("猫\r\nabc"), 0600); err != nil {
		t.Fatal(err)
	}
	xmlPath := filepath.Join(root, "selection.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{Main: []session.NPPFile{{Path: path, Encoding: -1, StartPos: 5, EndPos: 8}}}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	selection := state.space.Current().Editor.Selection()
	if selection != (text.Range{Start: 2, End: 5}) || state.space.Current().Editor.String() != "猫\nabc" {
		t.Fatalf("CRLF Unicode import selection = %+v content=%q", selection, state.space.Current().Editor.String())
	}
}

func TestSaveNPPSessionCannotOverwritePrivateAutoSession(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	privatePath := filepath.Join(root, "config", "session.json")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := session.Save(privatePath, session.State{}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, privatePath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueSaveFile(privatePath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(1))
	clickID(t, driver, saveSessionConfirmID)
	if after, err := os.ReadFile(privatePath); err != nil || string(after) != string(before) || !strings.Contains(state.notice, "internal") {
		t.Fatalf("private session overwritten: %q, %v, notice=%q", after, err, state.notice)
	}
}
