package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/document"
)

func TestFindAllOpenDocumentsIncludesUnsavedBuffers(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("猫 cat"); err != nil {
		t.Fatal(err)
	}
	first := state.space.Current()
	clickID(t, driver, newID)
	clickID(t, driver, editorID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	second := state.space.Current()
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(8))
	if !state.resultsOpen || !state.resultOpenDocuments || state.resultDocument != nil || len(state.fileResults.Matches) != 2 ||
		len(state.resultDocuments) != 2 || state.resultDocuments[0] != first || state.resultDocuments[1] != second {
		t.Fatalf("open-buffer results = %+v sources=%+v", state.fileResults, state.resultDocuments)
	}
	clickID(t, driver, resultScope.At(0))
	selection := first.Editor.Selection()
	if state.space.Current() != first || first.Path != "" || !first.Dirty() || second.Path != "" ||
		string([]rune(first.Editor.String())[selection.Start:selection.End]) != "cat" {
		t.Fatalf("unsaved result navigation = current:%+v selection:%+v", state.space.Current(), selection)
	}
}

func TestFindAllOpenDocumentsPreservesSplitOwnership(t *testing.T) {
	root := t.TempDir()
	firstPath, secondPath := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{firstPath, secondPath})
	first, second := state.space.Documents[0], state.space.Documents[1]
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{first, second}
	state.paneOwner[first], state.paneOwner[second] = 0, 1
	state.activePane = 1
	state.space.Select(1)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.query.Insert("cat")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(8))
	clickID(t, driver, resultScope.At(0))
	if state.activePane != 0 || state.paneDocs[0] != first || state.paneDocs[1] != second || state.paneOwner[first] != 0 || state.paneOwner[second] != 1 {
		t.Fatalf("open result moved a pane: active=%d panes=%+v owners=%+v", state.activePane, state.paneDocs, state.paneOwner)
	}
}

func TestFindAllOpenDocumentsCapsGloballyAndDropsClosedSource(t *testing.T) {
	state := newModel()
	first := state.space.Current()
	first.Editor.Insert(strings.Repeat("x ", 1000))
	second := state.space.NewDocument()
	second.Editor.Insert(strings.Repeat("x ", 1200))
	state.showInActivePane(second)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.query.Insert("x")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, menuItemScope.At(8))
	if len(state.fileResults.Matches) != 2000 || !state.fileResults.Truncated || state.resultDocuments[0] != first || state.resultDocuments[1999] != second {
		t.Fatalf("global result cap = %+v", state.fileResults)
	}
	clickID(t, driver, tabCloseScope.At(0))
	clickID(t, driver, discardID)
	if !state.resultsOpen || len(state.fileResults.Matches) != 1000 || len(state.resultDocuments) != 1000 {
		t.Fatalf("closed-source results = %+v sources=%d", state.fileResults, len(state.resultDocuments))
	}
	if !strings.Contains(state.notice, "1000 matches remain") || !strings.Contains(state.notice, "truncated") {
		t.Fatalf("closed-source status is stale: %q", state.notice)
	}
	for _, source := range state.resultDocuments {
		if source != second {
			t.Fatal("closed document remained in the result list")
		}
	}
	clickID(t, driver, resultScope.At(0))
	if state.space.Current() != second || state.notice == "Search result document is closed" {
		t.Fatalf("remaining source not navigable: %+v notice=%q", state.space.Current(), state.notice)
	}
}
