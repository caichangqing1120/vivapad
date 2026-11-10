package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/session"
)

func TestNPPSessionExportsVisibleFolderTreeState(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.Mkdir(src, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(src, "main.go")
	if err := os.WriteFile(file, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	canonical := state.folderRoots[0]
	state.folderExpanded[canonical] = true
	state.folderExpanded[filepath.Join(canonical, "src")] = true
	state.folderSelected = state.space.Current().Path
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	xmlPath := filepath.Join(root, "export.xml")
	if err := driver.QueueSaveFile(xmlPath, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Save Session...")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveSessionConfirmID)
	stored, err := session.LoadNPP(xmlPath)
	if err != nil || !stored.FileBrowserIncluded || len(stored.FolderRoots) != 1 || stored.FolderRoots[0].Path != canonical ||
		len(stored.FolderRoots[0].Expanded) != 2 || stored.FolderSelected != state.space.Current().Path {
		t.Fatalf("exported folder tree = %+v, %v, notice=%q", stored, err, state.notice)
	}
}

func TestNPPFolderOnlyImportPreservesDirtyTabAndRestoresExpansion(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.Mkdir(src, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(src, "note.txt")
	if err := os.WriteFile(file, []byte("note"), 0600); err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(root)
	xmlPath := filepath.Join(root, "folder-only.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{
		FileBrowserIncluded: true,
		FolderSelected:      filepath.Join(canonical, "src", "note.txt"),
		FolderRoots: []session.NPPFolderRoot{{Path: canonical, Expanded: []string{
			canonical, filepath.Join(canonical, "src"),
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	dirty := state.space.Current()
	dirty.Editor.Insert("keep draft")
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Load Session...")
	if len(state.space.Documents) != 1 || state.space.Current() != dirty || dirty.Editor.String() != "keep draft" ||
		!state.folderVisible || len(state.folderRoots) != 1 || state.folderSelected != filepath.Join(canonical, "src", "note.txt") {
		t.Fatalf("folder-only import = docs:%+v roots:%+v selected:%q notice:%q", state.space.Documents, state.folderRoots, state.folderSelected, state.notice)
	}
	found := false
	for _, row := range state.folderRows() {
		found = found || row.path == filepath.Join(canonical, "src", "note.txt")
	}
	if !found {
		t.Fatal("expanded imported folder did not expose its selected file")
	}
}

func TestNPPFolderPreflightRejectsMissingRootBeforeOpeningFiles(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "valid.txt")
	if err := os.WriteFile(file, []byte("valid"), 0600); err != nil {
		t.Fatal(err)
	}
	canonicalFile, _ := filepath.EvalSymlinks(file)
	xmlPath := filepath.Join(root, "invalid.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{
		Main:                []session.NPPFile{{Path: canonicalFile, Encoding: -1}},
		FileBrowserIncluded: true,
		FolderRoots:         []session.NPPFolderRoot{{Path: filepath.Join(root, "missing")}},
	}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	dirty := state.space.Current()
	dirty.Editor.Insert("draft")
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Load Session...")
	if len(state.space.Documents) != 1 || state.space.Current() != dirty || len(state.folderRoots) != 0 ||
		!strings.Contains(state.notice, "Folder") {
		t.Fatalf("failed folder preflight partially imported: docs=%+v roots=%+v notice=%q", state.space.Documents, state.folderRoots, state.notice)
	}
}

func TestNPPFolderPreflightRejectsSelectedSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup requires privileges on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	file := filepath.Join(outside, "outside.txt")
	if err := os.WriteFile(file, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias.txt")
	if err := os.Symlink(file, alias); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, _ := filepath.EvalSymlinks(root)
	xmlPath := filepath.Join(outside, "escape.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{
		FileBrowserIncluded: true, FolderSelected: filepath.Join(canonicalRoot, "alias.txt"),
		FolderRoots: []session.NPPFolderRoot{{Path: canonicalRoot}},
	}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Load Session...")
	if len(state.folderRoots) != 0 || state.folderVisible || !strings.Contains(state.notice, "outside") {
		t.Fatalf("symlink escape accepted: roots=%+v notice=%q", state.folderRoots, state.notice)
	}
}

func TestNPPFolderPreflightRejectsParentRootConflictBeforeOpeningFile(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "valid.txt")
	if err := os.WriteFile(file, []byte("valid"), 0600); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, _ := filepath.EvalSymlinks(root)
	canonicalChild, _ := filepath.EvalSymlinks(child)
	canonicalFile, _ := filepath.EvalSymlinks(file)
	xmlPath := filepath.Join(root, "conflict.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{
		Main:                []session.NPPFile{{Path: canonicalFile, Encoding: -1}},
		FileBrowserIncluded: true, FolderRoots: []session.NPPFolderRoot{{Path: canonicalRoot}},
	}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := state.addFolderRoot(canonicalChild); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	state.command("Load Session...")
	if len(state.space.Documents) != 1 || len(state.folderRoots) != 1 || state.folderRoots[0] != canonicalChild ||
		!strings.Contains(state.notice, "conflicts") {
		t.Fatalf("parent conflict imported partial state: docs=%+v roots=%+v notice=%q", state.space.Documents, state.folderRoots, state.notice)
	}
}

func TestNPPFolderExportRejectsStaleExpandedDirectory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "saved.txt")
	stale := filepath.Join(root, "removed")
	if err := os.WriteFile(file, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stale, 0700); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	if err := state.addFolderRoot(root); err != nil {
		t.Fatal(err)
	}
	state.folderExpanded[filepath.Join(state.folderRoots[0], "removed")] = true
	if err := os.Remove(stale); err != nil {
		t.Fatal(err)
	}
	if _, err := state.nppSessionState(); err == nil {
		t.Fatal("stale expanded folder was silently omitted from export")
	}
}
