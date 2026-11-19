package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/searchfiles"
)

func collectFinderSnapshot(t *testing.T, state *model, route, root string) {
	t.Helper()
	switch route {
	case "current":
		state.findAllCurrent()
	case "all-open":
		state.findAllOpenDocuments()
	case "directory":
		result, err := searchfiles.Find(context.Background(), root, state.query.String(), state.searchOptions, searchfiles.Options{})
		if err != nil {
			t.Fatal(err)
		}
		state.fileResults, state.resultsOpen, state.resultSelected = result, true, -1
	default:
		t.Fatalf("unknown collection route %q", route)
	}
	state.invalidator.Invalidate()
}

func TestFinderSourceSnapshotRejectsRepeatedText(t *testing.T) {
	t.Logf("Match footprint: %d bytes", unsafe.Sizeof(searchfiles.Match{}))
	for _, size := range [][2]int{{1280, 800}, {640, 400}} {
		for _, route := range []string{"current", "all-open", "directory"} {
			for _, action := range []string{"pointer", "F4", "Enter"} {
				for _, word := range []string{"cat", "\u732b"} {
					t.Run(fmt.Sprintf("%dx%d/%s/%s/%s", size[0], size[1], route, action, word), func(t *testing.T) {
						root := t.TempDir()
						path := filepath.Join(root, "source.txt")
						source := word + " " + word
						state, driver := startDriver(t)
						if err := driver.Resize(size[0], size[1]); err != nil {
							t.Fatal(err)
						}
						doc := state.space.Current()
						doc.Editor.Insert(source)
						if err := doc.Save(path); err != nil {
							t.Fatal(err)
						}
						state.query.Insert(word)
						collectFinderSnapshot(t, state, route, root)
						if err := driver.Pump(); err != nil {
							t.Fatal(err)
						}
						if len(state.fileResults.Matches) != 2 {
							t.Fatalf("fixture matches = %d", len(state.fileResults.Matches))
						}
						clickID(t, driver, resultScope.At(0))
						if state.resultSelected != 0 {
							t.Fatal("unchanged first result was not navigable")
						}
						clickID(t, driver, editorID)
						doc.Editor.MoveTo(0, false)
						doc.Editor.Insert(word + " ")
						doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: doc.Editor.RuneCount() - 1})
						if action == "Enter" {
							if err := driver.Key(gfx.KeyF7, 0); err != nil {
								t.Fatal(err)
							}
							if err := driver.Key(gfx.KeyDown, 0); err != nil {
								t.Fatal(err)
							}
							if driver.Snapshot().Focused != resultsPaneID || state.resultSelected != 1 {
								t.Fatal("Enter fixture did not focus the second result")
							}
						}
						before, caret, selected := doc.Editor.Selection(), doc.Editor.Caret(), state.resultSelected
						state.invalidator.Invalidate()
						if err := driver.Pump(); err != nil {
							t.Fatal(err)
						}
						switch action {
						case "pointer":
							clickID(t, driver, resultScope.At(1))
						case "F4":
							if err := driver.Key(gfx.KeyF4, 0); err != nil {
								t.Fatal(err)
							}
						case "Enter":
							if err := driver.Key(gfx.KeyEnter, 0); err != nil {
								t.Fatal(err)
							}
						}
						if state.space.Current() != doc || len(state.space.Documents) != 1 || state.resultSelected != selected ||
							doc.Editor.Selection() != before || doc.Editor.Caret() != caret ||
							doc.Editor.String() != word+" "+source || !doc.Dirty() || !doc.Editor.CanUndo() ||
							!strings.Contains(state.notice, "changed") {
							t.Fatalf("stale result changed its source: result=%d/%d selection=%+v/%+v caret=%d/%d notice=%q", state.resultSelected, selected, doc.Editor.Selection(), before, doc.Editor.Caret(), caret, state.notice)
						}
						if action == "Enter" && driver.Snapshot().Focused != resultsPaneID {
							t.Fatal("refused Enter moved focus out of the results pane")
						}
						if data, err := os.ReadFile(path); err != nil || string(data) != source {
							t.Fatalf("result navigation changed disk: %q, %v", data, err)
						}
						if !doc.Editor.Undo() || doc.Editor.String() != source || doc.Dirty() {
							t.Fatal("refusal did not preserve the saved Undo boundary")
						}
						if !state.openFileResult(1) || state.resultSelected != 1 || doc.Editor.Selection().End != doc.Editor.RuneCount() {
							t.Fatal("exact Undo-restored source was not navigable")
						}
					})
				}
			}
		}
	}
}

func TestFinderSourceSnapshotFreshSearchAndIndependentSources(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Editor.Insert("cat cat")
	second := state.space.NewDocument()
	second.Editor.Insert("cat second")
	state.showInActivePane(second)
	state.query.Insert("cat")
	state.findAllOpenDocuments()
	first.Editor.MoveTo(0, false)
	first.Editor.Insert("cat ")
	if !state.openFileResult(2) || state.space.Current() != second || second.Editor.Selection() != (text.Range{Start: 0, End: 3}) {
		t.Fatal("editing one source invalidated another source's results")
	}
	state.findAllOpenDocuments()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, resultScope.At(2))
	if state.space.Current() != first || first.Editor.Selection() != (text.Range{Start: 8, End: 11}) || state.resultSelected != 2 {
		t.Fatal("fresh search did not bind the changed source's new third occurrence")
	}
}

func TestFinderSourceSnapshotPreservesSplitOwnershipOnRefusal(t *testing.T) {
	for _, route := range []string{"current", "directory"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			state, driver := startDriver(t)
			first := state.space.Current()
			first.Editor.Insert("cat cat")
			if err := first.Save(filepath.Join(root, "source.txt")); err != nil {
				t.Fatal(err)
			}
			second := state.space.NewDocument()
			second.Editor.Insert("other")
			state.showInActivePane(second)
			state.toggleSplit()
			state.paneDocs = [2]*document.Document{first, second}
			state.paneOwner[first], state.paneOwner[second] = 0, 1
			state.activePane = 0
			state.space.Select(state.documentIndex(first))
			state.query.Insert("cat")
			collectFinderSnapshot(t, state, route, root)
			first.Editor.MoveTo(0, false)
			first.Editor.Insert("cat ")
			state.activePane = 1
			state.space.Select(state.documentIndex(second))
			before, caret := second.Editor.Selection(), second.Editor.Caret()
			recents := slices.Clone(state.recentEntries)
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, resultScope.At(1))
			if state.space.Current() != second || state.activePane != 1 || state.paneDocs != ([2]*document.Document{first, second}) ||
				state.paneOwner[first] != 0 || state.paneOwner[second] != 1 || second.Editor.Selection() != before ||
				second.Editor.Caret() != caret || state.resultSelected != -1 || !slices.Equal(state.recentEntries, recents) ||
				!strings.Contains(state.notice, "changed") {
				t.Fatal("stale result transferred split ownership or changed active editor/recent history")
			}
			if !first.Editor.Undo() || !state.openFileResult(1) || state.space.Current() != first || state.activePane != 0 ||
				state.paneDocs != ([2]*document.Document{first, second}) || state.paneOwner[first] != 0 || state.paneOwner[second] != 1 {
				t.Fatal("valid restored result transferred its document to the wrong pane")
			}
		})
	}
}

func TestFinderSourceSnapshotUnopenedDirectoryRefusalIsAtomic(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		t.Run(fmt.Sprintf("dirty=%v", dirty), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "source.txt")
			if err := os.WriteFile(path, []byte("cat cat"), 0600); err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			doc := state.space.Current()
			if dirty {
				doc.Editor.Insert("private unsaved")
			}
			state.query.Insert("cat")
			collectFinderSnapshot(t, state, "directory", root)
			if err := os.WriteFile(path, []byte("cat cat cat"), 0600); err != nil {
				t.Fatal(err)
			}
			before, caret := doc.Editor.Selection(), doc.Editor.Caret()
			value := doc.Editor.String()
			panes, owners, recents := state.paneDocs, len(state.paneOwner), slices.Clone(state.recentEntries)
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, resultScope.At(1))
			if state.space.Current() != doc || len(state.space.Documents) != 1 || state.paneDocs != panes || len(state.paneOwner) != owners ||
				doc.Editor.String() != value || doc.Editor.Selection() != before || doc.Editor.Caret() != caret || doc.Dirty() != dirty ||
				state.resultSelected != -1 || !slices.Equal(state.recentEntries, recents) || !strings.Contains(state.notice, "changed") {
				t.Fatal("stale unopened result changed tabs, source, selection or recent history")
			}
			if err := os.WriteFile(path, []byte("cat cat"), 0600); err != nil {
				t.Fatal(err)
			}
			if !state.openFileResult(1) || state.space.Current().Path != state.fileResults.Matches[1].Path || state.resultSelected != 1 ||
				state.space.Current().Editor.Selection() != (text.Range{Start: 4, End: 7}) {
				t.Fatal("valid detached source was not adopted after validation")
			}
			if dirty && (len(state.space.Documents) != 2 || doc.Editor.String() != "private unsaved" || !doc.Dirty()) {
				t.Fatal("valid directory open discarded the existing unsaved source")
			}
		})
	}
}

func TestFinderDirectoryResultUsesOpenFileIdentityAcrossCaseSpellings(t *testing.T) {
	testFinderDirectoryResultPathSpellings(t, "source.txt", "SOURCE.TXT")
}

func TestFinderDirectoryResultUsesOpenFileIdentityAcrossUnicodeSpellings(t *testing.T) {
	testFinderDirectoryResultPathSpellings(t, "caf\u00e9.txt", "cafe\u0301.txt")
}

func testFinderDirectoryResultPathSpellings(t *testing.T, name, alternate string) {
	t.Helper()
	root := t.TempDir()
	path, other := filepath.Join(root, name), filepath.Join(root, alternate)
	const saved = "cat cat\r\n"
	if err := os.WriteFile(path, []byte(saved), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(other)
	alias := err == nil
	if !alias {
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.WriteFile(other, []byte(saved), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{other})
	opened := state.space.Current()
	if opened.Path == "" {
		t.Fatal("fixture did not open the file")
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.query.Insert("cat")
	collectFinderSnapshot(t, state, "directory", root)
	index := slices.IndexFunc(state.fileResults.Matches, func(match searchfiles.Match) bool {
		return filepath.Base(match.Path) == name && match.Start == 4
	})
	if index < 0 {
		t.Fatalf("fixture did not find the second occurrence in %q", name)
	}
	if alias && opened.Path == state.fileResults.Matches[index].Path {
		t.Skip("this filesystem resolves both spellings to the same path string")
	}
	opened.Editor.MoveTo(0, false)
	opened.Editor.Insert("cat ")
	opened.Editor.SetSelection(text.Range{Start: 3, End: 1})
	selection, caret, draft := opened.Editor.Selection(), opened.Editor.Caret(), opened.Editor.String()
	recents := slices.Clone(state.recentEntries)
	panes := state.paneDocs
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, resultScope.At(index))
	if alias {
		if len(state.space.Documents) != 1 || state.space.Current() != opened || state.paneDocs != panes ||
			state.resultSelected != -1 || state.notice != staleSearchResultNotice || !slices.Equal(state.recentEntries, recents) {
			t.Fatalf("stale aliased directory result opened another document: tabs=%d selection=%d notice=%q", len(state.space.Documents), state.resultSelected, state.notice)
		}
	} else if len(state.space.Documents) != 2 || state.space.Current() == opened || state.resultSelected != index ||
		filepath.Base(state.space.Current().Path) != name {
		t.Fatal("distinct files were treated as aliases")
	}
	if opened.Editor.String() != draft || opened.Editor.Selection() != selection || opened.Editor.Caret() != caret || !opened.Dirty() {
		t.Fatal("result activation changed the open draft, caret or selection")
	}
	if alias {
		if !opened.Editor.Undo() || !state.openFileResult(index) || len(state.space.Documents) != 1 || state.space.Current() != opened ||
			opened.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
			t.Fatal("restoring the snapshot did not navigate in the existing tab")
		}
	}
	for _, name := range []string{path, other} {
		if disk, err := os.ReadFile(name); err != nil || string(disk) != saved {
			t.Fatalf("result activation changed %q: %q, %v", name, disk, err)
		}
	}
}

func TestFinderDoesNotFollowRedirectedDirectoryResult(t *testing.T) {
	root := t.TempDir()
	searchRoot := filepath.Join(root, "search")
	if err := os.Mkdir(searchRoot, 0700); err != nil {
		t.Fatal(err)
	}
	path, other := filepath.Join(searchRoot, "source.txt"), filepath.Join(root, "other.txt")
	const contents = "prefix cat\r\n"
	for _, name := range []string{path, other} {
		if err := os.WriteFile(name, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("unsaved draft"); err != nil {
		t.Fatal(err)
	}
	draft := state.space.Current()
	draft.Editor.SetSelection(text.Range{Start: 0, End: 7})
	before := draft.Editor.Selection()
	recents := slices.Clone(state.recentEntries)
	state.query.Insert("cat")
	collectFinderSnapshot(t, state, "directory", searchRoot)
	if len(state.fileResults.Matches) != 1 {
		t.Fatal("fixture did not collect the original file")
	}
	parked := filepath.Join(root, "original.txt")
	if err := os.Rename(path, parked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, path); err != nil {
		if os.IsPermission(err) {
			t.Skipf("file symlinks are unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if state.space.Current() != draft || len(state.space.Documents) != 1 || state.resultSelected != -1 ||
		draft.Editor.String() != "unsaved draft" || draft.Editor.Selection() != before || !draft.Dirty() ||
		!slices.Equal(state.recentEntries, recents) || state.notice != staleSearchResultNotice {
		t.Fatalf("redirected result changed workspace or navigation: tabs=%d selected=%d notice=%q", len(state.space.Documents), state.resultSelected, state.notice)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parked, path); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	if opened := state.space.Current(); opened == draft || opened.Path != state.fileResults.Matches[0].Path ||
		opened.Editor.Selection() != (text.Range{Start: 7, End: 10}) || state.resultSelected != 0 || state.notice != "" {
		t.Fatalf("restored path did not open its original match: %q, selected=%d notice=%q", opened.Path, state.resultSelected, state.notice)
	}
	for _, name := range []string{path, other} {
		if data, err := os.ReadFile(name); err != nil || string(data) != contents {
			t.Fatalf("result activation changed %q: %q, %v", name, data, err)
		}
	}
}

func TestFinderMissingDirectoryResultShowsCauseAndStaysRetryable(t *testing.T) {
	root := filepath.Join(t.TempDir(), strings.Repeat("long-directory-", 8))
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "source.txt")
	source := []byte("prefix cat\n")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("unsaved draft"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	before, caret := doc.Editor.Selection(), doc.Editor.Caret()
	recents := slices.Clone(state.recentEntries)
	state.query.Insert("cat")
	collectFinderSnapshot(t, state, "directory", root)
	if len(state.fileResults.Matches) != 1 {
		t.Fatal("fixture did not collect the unopened result")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	snapshot := driver.Snapshot()
	if state.resultSelected != -1 || !state.resultsOpen || state.space.Current() != doc || len(state.space.Documents) != 1 ||
		snapshot.Focused != editorID || doc.Editor.String() != "unsaved draft" || !doc.Dirty() || !doc.Editor.CanUndo() ||
		doc.Editor.Selection() != before || doc.Editor.Caret() != caret || !slices.Equal(state.recentEntries, recents) {
		t.Fatal("missing result changed the editor, result position, tabs or recent files")
	}
	visible := false
	for _, label := range snapshot.Texts {
		visible = visible || strings.Contains(label.Value, "Search result file is missing")
	}
	if !visible {
		t.Errorf("compact status hid the reason the result could not open: %q", state.notice)
	}
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyF4, 0); err != nil {
		t.Fatal(err)
	}
	opened := state.space.Current()
	if opened == doc || opened.Path != state.fileResults.Matches[0].Path || state.resultSelected != 0 ||
		opened.Editor.Selection() != (text.Range{Start: 7, End: 10}) || opened.Dirty() ||
		doc.Editor.String() != "unsaved draft" || !doc.Dirty() {
		t.Fatal("retry did not open the restored match while preserving the draft")
	}
	if state.notice != "" {
		t.Errorf("successful retry left a failure notice: %q", state.notice)
	}
}

func TestFinderSourceSnapshotDirectoryKeepsEncodingMismatchRefusal(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.txt")
	state, _ := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	other := state.space.NewDocument()
	other.Editor.Insert("private unsaved")
	state.showInActivePane(other)
	result, err := searchfiles.Find(context.Background(), root, "cat", document.SearchOptions{}, searchfiles.Options{Encoding: document.Windows1252})
	if err != nil || len(result.Matches) != 2 {
		t.Fatalf("legacy directory fixture: %+v, %v", result, err)
	}
	state.fileResults, state.resultsOpen = result, true
	before, caret, recents := other.Editor.Selection(), other.Editor.Caret(), slices.Clone(state.recentEntries)
	if state.openFileResult(1) || state.space.Current() != other || len(state.space.Documents) != 2 ||
		other.Editor.Selection() != before || other.Editor.Caret() != caret || !other.Dirty() ||
		state.resultSelected != -1 || !slices.Equal(state.recentEntries, recents) || !strings.Contains(state.notice, "already open as") {
		t.Fatal("directory result lost the existing explicit-encoding refusal")
	}
}

func TestFinderDirectoryResultKeepsSourceDecoding(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(fmt.Sprintf("pendingUTF8=%t", pending), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "source.txt")
			const contents = "cat cat\r\n"
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			state, _ := startDriver(t)
			if err := state.space.OpenWithEncoding(path, document.Windows1252); err != nil {
				t.Fatal(err)
			}
			target := state.space.Current()
			if pending {
				target.SetEncoding(document.UTF8)
			}
			from := state.space.NewDocument()
			from.Editor.Insert("private draft")
			state.showInActivePane(from)
			before := from.Editor.Selection()
			targetBefore := target.Editor.Selection()
			recents := slices.Clone(state.recentEntries)
			state.query.Insert("cat")
			collectFinderSnapshot(t, state, "directory", root)
			if matches := state.fileResults.Matches; len(matches) != 2 || matches[1].Encoding != document.UTF8 || matches[1].Source != target.Editor.String() {
				t.Fatalf("fixture must have equal text from a different decoder: %+v", matches)
			}
			if state.openFileResult(1) {
				t.Error("Unicode-auto result navigated to a tab decoded as Windows-1252")
			}
			if state.space.Current() != from || len(state.space.Documents) != 2 || from.Editor.Selection() != before ||
				from.Editor.String() != "private draft" || !from.Editor.CanUndo() || target.Editor.Selection() != targetBefore ||
				state.resultSelected != -1 || !state.resultsOpen || !slices.Equal(state.recentEntries, recents) {
				t.Error("refused activation changed the editor, result position or recent files")
			}
			if target.Dirty() != pending || target.SavedEncoding() != document.Windows1252 {
				t.Error("result activation changed the pending save conversion")
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != contents {
				t.Errorf("result activation changed disk bytes: %q, %v", data, err)
			}

			// Keep the same directory and text; use the tab's actual decoder.
			target.SetEncoding(document.Windows1252)
			result, err := searchfiles.Find(context.Background(), root, "cat", document.SearchOptions{}, searchfiles.Options{Encoding: document.Windows1252})
			if err != nil || len(result.Matches) != 2 {
				t.Fatalf("legacy directory fixture: %+v, %v", result, err)
			}
			state.fileResults = result
			if !state.openFileResult(1) || state.space.Current() != target || state.resultSelected != 1 ||
				target.Editor.Selection() != (text.Range{Start: 4, End: 7}) || state.notice != "" {
				t.Errorf("matching decoder did not navigate to the second match: %q", state.notice)
			}
		})
	}
}

func TestFinderUnopenedDirectoryResultRejectsChangedUnicodeEncoding(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.txt")
	contents := []byte("cat cat\r\n")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	state, _ := startDriver(t)
	from := state.space.Current()
	from.Editor.Insert("private draft")
	state.query.Insert("cat")
	collectFinderSnapshot(t, state, "directory", root)
	if err := os.WriteFile(path, []byte("\xef\xbb\xbfcat cat\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if state.openFileResult(1) {
		t.Fatal("a new Unicode encoding reused the earlier directory result")
	}
	if state.space.Current() != from || len(state.space.Documents) != 1 || state.resultSelected != -1 || !state.resultsOpen {
		t.Fatal("changed encoding altered tabs or result position")
	}
	collectFinderSnapshot(t, state, "directory", root)
	if !state.openFileResult(1) || state.space.Current().SavedEncoding() != document.UTF8BOM ||
		state.space.Current().Editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("fresh BOM result did not select the second match: %q", state.notice)
	}
}
