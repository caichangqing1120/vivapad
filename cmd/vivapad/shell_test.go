package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/preferences"
	"vivapad/internal/recovery"
	"vivapad/internal/searchfiles"
	"vivapad/internal/session"
	"vivapad/internal/syntax"
)

func startDriver(t *testing.T) (*model, *app.Driver) {
	t.Helper()
	state := newModel()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return state, driver
}

func clickID(t *testing.T, driver *app.Driver, id gfx.ID) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			bounds := hit.Bounds
			if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	for index := range menuCommands[0] {
		if id == menuItemScope.At(index) && fileMenuControls(driver) {
			bounds := revealFileMenuItem(t, driver, index)
			if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	for index := range menuCommands[3] {
		if id == menuItemScope.At(index) {
			if viewMenuControls(driver) {
				bounds := revealViewMenuItem(t, driver, index)
				if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
					t.Fatal(err)
				}
				return
			}
		}
	}
	for index := range menuCommands[1] {
		if id == menuItemScope.At(index) && editMenuControls(driver) {
			bounds := revealEditMenuItem(t, driver, index)
			if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	for index := range menuCommands[2] {
		if id == menuItemScope.At(index) && searchMenuControls(driver) {
			bounds := revealSearchMenuItem(t, driver, index)
			if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("missing hit %v", id)
}

func searchMenuControls(driver *app.Driver) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == searchMenuNextID {
			return true
		}
	}
	return false
}

func revealSearchMenuItem(t *testing.T, driver *app.Driver, index int) gfx.Rect {
	t.Helper()
	for range len(menuCommands[2]) + 1 {
		first, last := len(menuCommands[2]), -1
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == menuItemScope.At(index) {
				return hit.Bounds
			}
			for visible := range menuCommands[2] {
				if hit.ID == menuItemScope.At(visible) {
					first, last = min(first, visible), max(last, visible)
				}
			}
		}
		switch {
		case index < first:
			clickID(t, driver, searchMenuPreviousID)
		case index > last:
			clickID(t, driver, searchMenuNextID)
		default:
			t.Fatalf("Search item %d is not reachable", index)
		}
	}
	t.Fatalf("Search item %d did not become visible", index)
	return gfx.Rect{}
}

func editMenuControls(driver *app.Driver) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == editMenuNextID {
			return true
		}
	}
	return false
}

func revealEditMenuItem(t *testing.T, driver *app.Driver, index int) gfx.Rect {
	t.Helper()
	for range len(menuCommands[1]) + 1 {
		first, last := len(menuCommands[1]), -1
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == menuItemScope.At(index) {
				return hit.Bounds
			}
			for visible := range menuCommands[1] {
				if hit.ID == menuItemScope.At(visible) {
					first, last = min(first, visible), max(last, visible)
				}
			}
		}
		if index < first {
			clickID(t, driver, editMenuPreviousID)
		} else if index > last {
			clickID(t, driver, editMenuNextID)
		} else {
			t.Fatalf("Edit item %d is not reachable", index)
		}
	}
	t.Fatalf("Edit item %d did not become visible", index)
	return gfx.Rect{}
}

func editMenuIndex(t *testing.T, label string) int {
	t.Helper()
	index := slices.Index(menuCommands[1], label)
	if index < 0 {
		t.Fatalf("Edit menu does not offer %s", label)
	}
	return index
}

func clickEditMenuCommand(t *testing.T, driver *app.Driver, label string) gfx.Rect {
	t.Helper()
	index := editMenuIndex(t, label)
	bounds := revealEditMenuItem(t, driver, index)
	clickID(t, driver, menuItemScope.At(index))
	return bounds
}

func viewMenuControls(driver *app.Driver) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == viewMenuNextID {
			return true
		}
	}
	return false
}

func revealViewMenuItem(t *testing.T, driver *app.Driver, index int) gfx.Rect {
	t.Helper()
	for range len(menuCommands[3]) + 1 {
		first, last := len(menuCommands[3]), -1
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == menuItemScope.At(index) {
				return hit.Bounds
			}
			for visible := range menuCommands[3] {
				if hit.ID == menuItemScope.At(visible) {
					first, last = min(first, visible), max(last, visible)
				}
			}
		}
		switch {
		case index < first:
			clickID(t, driver, viewMenuPreviousID)
		case index > last:
			clickID(t, driver, viewMenuNextID)
		default:
			t.Fatalf("View item %d missing in visible rows %d-%d", index, first, last)
		}
	}
	t.Fatalf("View item %d unreachable", index)
	return gfx.Rect{}
}

func TestTypeSaveAndOpenThroughToolbar(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("Hello 世界"); err != nil {
		t.Fatal(err)
	}
	if got := state.space.Current().Editor.String(); got != "Hello 世界" {
		t.Fatalf("typed text = %q", got)
	}
	path := filepath.Join(t.TempDir(), "hello.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveID)
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "Hello 世界" || state.space.Current().Dirty() {
		t.Fatalf("saved = %q, %v, dirty=%v", data, err, state.space.Current().Dirty())
	}
	clickID(t, driver, newID)
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, openID)
	if state.space.Current().Path == "" || state.space.Current().Editor.String() != "Hello 世界" {
		t.Fatalf("opened document = %+v", state.space.Current())
	}
}

func TestCanceledSaveAndDirtyClose(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("keep me"); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveID)
	if state.space.Current().Path != "" || !state.space.Current().Dirty() {
		t.Fatal("canceled save changed document")
	}
	clickID(t, driver, tabCloseScope.At(state.tabHitIndex(state.space.Documents[0])))
	if state.pendingClose != 0 {
		t.Fatal("dirty tab did not ask for confirmation")
	}
	clickID(t, driver, cancelCloseID)
	if state.pendingClose != -1 || state.space.Current().Editor.String() != "keep me" {
		t.Fatal("cancel did not preserve the document")
	}
	clickID(t, driver, tabCloseScope.At(state.tabHitIndex(state.space.Documents[0])))
	clickID(t, driver, discardID)
	if state.space.Current().Editor.String() != "" || state.pendingClose != -1 {
		t.Fatal("explicit discard did not close the document")
	}
}

func TestSearchReplaceAndKeyboardNew(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("one TWO one"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchID)
	clickID(t, driver, queryID)
	if err := driver.Type("one"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, findNextID)
	if got := state.space.Current().Editor.Selection(); got.Start != 0 || got.End != 3 {
		t.Fatalf("search selection = %+v", got)
	}
	clickID(t, driver, replacementID)
	if err := driver.Type("three"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replaceAllID)
	if got := state.space.Current().Editor.String(); got != "three TWO three" {
		t.Fatalf("replace result = %q", got)
	}
	if _, err := driver.ShortcutChord(primary(), 'n'); err != nil {
		t.Fatal(err)
	}
	if state.space.Current().Name != "new 2" {
		t.Fatalf("shortcut did not create tab: %+v", state.space.Current())
	}
}

func TestOpenPickerCancellationLeavesTabsUntouched(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.QueueOpenFiles(nil, errors.New("dialog canceled")); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, openID)
	if len(state.space.Documents) != 1 || state.space.Current().Path != "" {
		t.Fatal("canceled picker changed tabs")
	}
}

func TestCommentsOnlyHighlightInSupportedLanguages(t *testing.T) {
	var highlighter syntax.Highlighter
	if got := highlighter.Highlight(".txt", "# plain text", nil); len(got) != 0 {
		t.Fatalf("plain text highlight = %+v", got)
	}
	if got := highlighter.Highlight(".go", "// comment", nil); len(got) == 0 || got[0].Start != 0 || got[0].End != len("// comment") {
		t.Fatalf("Go comment highlight = %+v", got)
	}
}

func TestWindowCloseRequestsConfirmationForDirtyDocument(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("unsaved")
	allowed := false
	config := newConfig(state)
	config.OnCloseRequest(func() { allowed = true })
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if allowed || state.pendingClose != 0 {
		t.Fatal("window close discarded unsaved content")
	}
	found := false
	for _, hit := range driver.Snapshot().Hits {
		found = found || hit.ID == discardID
	}
	if !found {
		t.Fatal("close confirmation did not repaint")
	}
	clickID(t, driver, cancelCloseID)
	if allowed || state.space.Current().Editor.String() != "unsaved" {
		t.Fatal("canceling window close lost content")
	}
}

func TestClickOutsideMenuClosesWithoutChangingDocument(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(0))
	if state.menuOpen != 0 {
		t.Fatal("File menu did not open")
	}
	if err := driver.Click(500, 400); err != nil {
		t.Fatal(err)
	}
	if state.menuOpen != -1 || len(state.space.Documents) != 1 {
		t.Fatalf("outside click = menu %d, tabs %d", state.menuOpen, len(state.space.Documents))
	}
}

func TestDirtyCloseDialogBlocksUnderlyingToolbar(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("unsaved")
	clickID(t, driver, tabCloseScope.At(state.tabHitIndex(state.space.Documents[0])))
	if state.pendingClose != 0 {
		t.Fatal("missing confirmation")
	}
	clickID(t, driver, newID)
	if len(state.space.Documents) != 1 || state.pendingClose != 0 {
		t.Fatal("clicked toolbar through confirmation")
	}
}

func TestDirtyCloseDialogBlocksEditingAndShortcuts(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("before"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, tabCloseScope.At(state.tabHitIndex(state.space.Documents[0])))
	_ = driver.Type("after")
	if _, err := driver.ShortcutChord(primary(), 'n'); err != nil {
		t.Fatal(err)
	}
	if len(state.space.Documents) != 1 || state.space.Current().Editor.String() != "before" {
		t.Fatalf("modal leaked input: %+v, %q", state.space, state.space.Current().Editor.String())
	}
}

func TestStartupOpensFileArgument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "startup.txt")
	if err := os.WriteFile(path, []byte("from argument"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	if state.space.Current().Editor.String() != "from argument" || state.space.Current().Name != "startup.txt" {
		t.Fatalf("startup document = %+v", state.space.Current())
	}
}

func TestStartupRestoresSessionOnlyWithoutFileArguments(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, second, explicit := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt"), filepath.Join(root, "explicit.txt")
	for _, path := range []string{first, second, explicit} {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := session.Save(sessionPath, session.State{Paths: []string{first, second}, Active: first}); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, sessionPath)
	if len(restored.space.Documents) != 2 || restored.space.Current().Name != "first.txt" {
		t.Fatalf("restored tabs=%d active=%s", len(restored.space.Documents), restored.space.Current().Name)
	}
	withArguments := newStartupModel([]string{explicit}, sessionPath)
	if len(withArguments.space.Documents) != 1 || withArguments.space.Current().Name != "explicit.txt" {
		t.Fatalf("explicit path was not preferred: %+v", withArguments.space.Current())
	}
}

func TestCanceledCloseDoesNotPersistSession(t *testing.T) {
	state := newStartupModel(nil, filepath.Join(t.TempDir(), "settings", "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.space.Current().Editor.Insert("unsaved")
	config := newConfig(state)
	config.OnCloseRequest(func() { t.Fatal("dirty close was allowed") })
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, cancelCloseID)
	if _, err := os.Stat(state.sessionPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancel wrote session, stat=%v", err)
	}
}

func TestWindowClosePersistsSavedAndDiscardedTabs(t *testing.T) {
	root := t.TempDir()
	state := newStartupModel(nil, filepath.Join(root, "settings", "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.space.Current().Editor.Insert("saved")
	state.space.NewDocument().Editor.Insert("discarded")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	allowed := false
	config := newConfig(state)
	config.OnCloseRequest(func() { allowed = true })
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	savedPath := filepath.Join(root, "saved.txt")
	if err := driver.QueueSaveFile(savedPath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveCloseID)
	clickID(t, driver, discardID)
	stored, err := session.Load(state.sessionPath)
	canonical, pathErr := filepath.EvalSymlinks(savedPath)
	if err != nil || pathErr != nil || !allowed || len(stored.Paths) != 1 || stored.Paths[0] != canonical {
		t.Fatalf("close session=%+v err=%v allowed=%v", stored, err, allowed)
	}
}

func TestCleanWindowCloseRestoresSelectedTab(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte(path), 0644); err != nil {
			t.Fatal(err)
		}
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{first, second}, sessionPath)
	state.space.Select(0)
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	restored := newStartupModel(nil, sessionPath)
	if !allowed || len(restored.space.Documents) != 2 || restored.space.Current().Name != "first.txt" {
		t.Fatalf("clean close restored tabs=%d active=%q allowed=%v", len(restored.space.Documents), restored.space.Current().Name, allowed)
	}
}

func TestSplitSessionRestoresPerViewTabsAndDivider(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt"), filepath.Join(root, "third.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel(paths, sessionPath)
	state.toggleSplit()
	state.paneDocs[0] = state.space.Documents[0]
	state.paneOwner[state.space.Documents[1]] = 1
	state.splitRatio = 0.37
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	restored := newStartupModel(nil, sessionPath)
	if !allowed || !restored.splitEnabled || restored.activePane != 1 || restored.paneDocs[0].Name != "first.txt" ||
		restored.paneDocs[1].Name != "third.txt" || restored.paneOwner[restored.space.Documents[1]] != 1 || restored.splitRatio != 0.37 {
		t.Fatalf("restored split = %+v owners=%+v ratio=%g active=%d", restored.paneDocs, restored.paneOwner, restored.splitRatio, restored.activePane)
	}
	driver, err := app.NewDriver(newConfig(restored), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if focus := driver.Snapshot().Focused; focus != rightEditorID {
		t.Fatalf("restored keyboard focus = %v, want right editor", focus)
	}
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if left, right := restored.paneDocs[0].Editor.String(), restored.paneDocs[1].Editor.String(); left != "first.txt" || right != "xthird.txt" {
		t.Fatalf("first key after reopening: left=%q right=%q", left, right)
	}
}

func TestRecoveredUntitledBufferReturnsToRightView(t *testing.T) {
	root := t.TempDir()
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel(nil, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	clickID(t, driver, rightPaneID)
	clickID(t, driver, newID)
	clickID(t, driver, rightEditorID)
	if err := driver.Type("right draft"); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	driver.Close()
	restored := newStartupModel(nil, sessionPath)
	if !restored.splitEnabled || restored.activePane != 1 || restored.paneDocs[1].Editor.String() != "right draft" ||
		restored.paneOwner[restored.paneDocs[1]] != 1 || !restored.paneDocs[1].Dirty() {
		t.Fatalf("recovered right draft = %+v active=%d", restored.paneDocs, restored.activePane)
	}
	left, right := restored.paneDocument(0), restored.paneDocument(1)
	if left == right || left.Path != "" || left.Dirty() || left.Editor.String() != "" || restored.paneOwner[left] != 0 {
		t.Fatalf("left pane did not recover as an independent blank tab: name=%q content=%q owner=%d",
			left.Name, left.Editor.String(), restored.paneOwner[left])
	}
	if restored.space.Current() != right {
		t.Fatal("restoring an empty left pane changed the active editor")
	}
}

func TestSavingDirtySplitTabsPreservesTheirViews(t *testing.T) {
	root := t.TempDir()
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel(nil, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, editorID)
	if err := driver.Type("left draft"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	clickID(t, driver, rightPaneID)
	clickID(t, driver, newID)
	clickID(t, driver, rightEditorID)
	if err := driver.Type("right draft"); err != nil {
		t.Fatal(err)
	}
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	leftPath, rightPath := filepath.Join(root, "left.txt"), filepath.Join(root, "right.txt")
	if err := driver.QueueSaveFile(leftPath, nil); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueSaveFile(rightPath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveCloseID)
	clickID(t, driver, saveCloseID)
	restored := newStartupModel(nil, sessionPath)
	if !allowed || !restored.splitEnabled || restored.paneDocs[0].Name != "left.txt" || restored.paneDocs[1].Name != "right.txt" ||
		restored.paneOwner[restored.paneDocs[1]] != 1 || restored.activePane != 1 {
		t.Fatalf("saved split recovery = %+v active=%d allowed=%v", restored.paneDocs, restored.activePane, allowed)
	}
}

func TestAbnormalStopRestoresDirtyFileWithoutOverwritingDisk(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "original.txt")
	if err := os.WriteFile(path, []byte("saved"), 0644); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{path}, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	state.space.Current().Editor.MoveTo(state.space.Current().Editor.RuneCount(), false)
	clickID(t, driver, editorID)
	if err := driver.Type(" edit"); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	driver.Close()
	restored := newStartupModel(nil, sessionPath)
	doc := restored.space.Current()
	if doc.Editor.String() != "saved edit" || !doc.Dirty() || doc.Path == "" {
		t.Fatalf("recovered = path:%q content:%q dirty:%v", doc.Path, doc.Editor.String(), doc.Dirty())
	}
	if disk, _ := os.ReadFile(path); string(disk) != "saved" {
		t.Fatalf("recovery overwrote disk: %q", disk)
	}
	if err := os.WriteFile(path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	conflicted := newStartupModel(nil, sessionPath).space.Current()
	if conflicted.Path != "" || !strings.HasPrefix(conflicted.Name, "Recovered ") || conflicted.Editor.String() != "saved edit" || !conflicted.Dirty() {
		t.Fatalf("conflicting recovery = path:%q name:%q content:%q dirty:%v", conflicted.Path, conflicted.Name, conflicted.Editor.String(), conflicted.Dirty())
	}
}

func TestNormalDiscardClearsRecoveryBackup(t *testing.T) {
	root := t.TempDir()
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel(nil, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, editorID)
	if err := driver.Type("discard me"); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(filepath.Dir(sessionPath), "recovery.json")
	if stored, err := recovery.Load(backupPath); err != nil || len(stored.Tabs) != 1 {
		t.Fatalf("pending backup = %+v, %v", stored, err)
	}
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, discardID)
	if _, err := os.Stat(backupPath); !allowed || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("discard backup still exists: allowed=%v err=%v", allowed, err)
	}
}

func TestCleanStartupDoesNotCreateRecoveryFile(t *testing.T) {
	root := t.TempDir()
	state := newStartupModel(nil, filepath.Join(root, "settings", "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(state.recoveryPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clean launch created a recovery file: %v", err)
	}
}

func TestExplicitArgumentDoesNotHideUnsavedRecovery(t *testing.T) {
	root := t.TempDir()
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := recovery.Save(filepath.Join(filepath.Dir(sessionPath), "recovery.json"), recovery.State{
		Active: 0, Tabs: []recovery.Tab{{Name: "new 9", Content: "unsaved text"}},
	}); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(root, "explicit.txt")
	if err := os.WriteFile(explicit, []byte("on disk"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{explicit}, sessionPath)
	if len(state.space.Documents) != 2 || state.space.Current().Name != "explicit.txt" ||
		state.space.Documents[1].Editor.String() != "unsaved text" || !state.space.Documents[1].Dirty() {
		t.Fatalf("explicit and recovered tabs = %+v", state.space)
	}
}

func TestRecoveryWriteFailureIsVisible(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, filepath.Join(blocked, "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, editorID)
	if err := driver.Type("critical unsaved text"); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err == nil {
		t.Fatal("backup unexpectedly succeeded")
	}
	deadline := time.Now().Add(2 * time.Second)
	for !strings.HasPrefix(state.notice, "Recovery failed:") && time.Now().Before(deadline) {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if !strings.HasPrefix(state.notice, "Recovery failed:") {
		t.Fatalf("backup failure not surfaced: %q", state.notice)
	}
}

func TestLongLineKeepsCaretVisible(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type(strings.Repeat("longword ", 140)); err != nil {
		t.Fatal(err)
	}
	caret, ok := state.codeEditor().CaretBounds()
	if !ok || caret.X < 10 || caret.X > 1270 || caret.Y < menuHeight+toolbarHeight+tabHeight || caret.Y >= 775 {
		t.Fatalf("caret not visible: %+v, %v", caret, ok)
	}
	bar := false
	for _, hit := range driver.Snapshot().Hits {
		bar = bar || hit.ID == horizontalBarID
	}
	if !bar {
		t.Fatal("long unwrapped line has no horizontal scrollbar")
	}
}

func TestToolbarUndoRedoAndClipboard(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("hello"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, undoID)
	if state.space.Current().Editor.String() != "" {
		t.Fatal("toolbar undo did not restore empty text")
	}
	clickID(t, driver, redoID)
	if state.space.Current().Editor.String() != "hello" {
		t.Fatal("toolbar redo did not restore typing")
	}
	state.space.Current().Editor.SetSelection(text.Range{Start: 0, End: 5})
	clickID(t, driver, copyID)
	if driver.Clipboard() != "hello" || state.space.Current().Editor.String() != "hello" {
		t.Fatal("copy changed text or missed clipboard")
	}
	clickID(t, driver, cutID)
	if state.space.Current().Editor.String() != "" || driver.Clipboard() != "hello" {
		t.Fatal("cut did not move selection to clipboard")
	}
	clickID(t, driver, pasteID)
	if state.space.Current().Editor.String() != "hello" {
		t.Fatal("paste did not restore clipboard text")
	}
}

func TestCompactWindowKeepsMenuInsideViewport(t *testing.T) {
	_, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	for _, label := range driver.Snapshot().Texts {
		if label.Bounds.Y < menuHeight && label.Bounds.X+label.Bounds.Width > 640 {
			t.Fatalf("menu label extends beyond 640px: %+v", label)
		}
	}
	clickID(t, driver, menuScope.At(0))
	found := false
	for _, hit := range driver.Snapshot().Hits {
		found = found || hit.ID == menuItemScope.At(0)
	}
	if !found {
		t.Fatal("File menu unavailable at minimum width")
	}
}

func TestOverflowTabsCanNavigateBackToFirst(t *testing.T) {
	state, driver := startDriver(t)
	for range 10 {
		clickID(t, driver, newID)
	}
	for range 10 {
		clickID(t, driver, tabLeftID)
	}
	clickID(t, driver, tabScope.At(state.tabHitIndex(state.space.Documents[0])))
	if state.space.Active != 0 || state.space.Current().Name != "new 1" {
		t.Fatalf("could not revisit first tab: active=%d", state.space.Active)
	}
}

func BenchmarkPaintIdleWindow(b *testing.B) {
	state := newModel()
	var canvas gfx.Canvas
	size := gfx.Size{Width: 1280, Height: 800}
	state.paint(&canvas, size)
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		state.paint(&canvas, size)
	}
}

func BenchmarkPaintHighlightedGoFile(b *testing.B) {
	state := newModel()
	state.space.Current().Name = "sample.go"
	state.space.Current().Editor.Insert(strings.Repeat("func main() { // sample\n", 400))
	var canvas gfx.Canvas
	size := gfx.Size{Width: 1280, Height: 800}
	state.paint(&canvas, size)
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		state.paint(&canvas, size)
	}
}

func TestSearchOptionsAndRegexReplacementThroughPanel(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("Foo foo item-12"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchID)
	clickID(t, driver, matchCaseID)
	clickID(t, driver, queryID)
	if err := driver.Type("foo"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, findNextID)
	if got := state.space.Current().Editor.Selection(); got.Start != 4 || got.End != 7 {
		t.Fatalf("match-case selection = %+v", got)
	}
	clickID(t, driver, queryID)
	if err := driver.SelectText(0, 3); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type(`(item)-(\d+)`); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, regexID)
	clickID(t, driver, replacementID)
	if err := driver.Type("$2:$1"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replaceAllID)
	if got := state.space.Current().Editor.String(); got != "Foo foo 12:item" {
		t.Fatalf("regex panel result = %q; notice=%q", got, state.notice)
	}
}

func TestGoToLineAndLineCommandsThroughShortcuts(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("first\nsecond\nthird"); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.ShortcutChord(primary(), 'g'); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, goToFieldID)
	if err := driver.Type("2"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, goToButtonID)
	if line, _ := state.space.Current().Position(); line != 2 {
		t.Fatalf("go-to selected line %d", line)
	}
	if _, err := driver.ShortcutChord(primary(), 'd'); err != nil {
		t.Fatal(err)
	}
	if got := state.space.Current().Editor.String(); got != "first\nsecond\nsecond\nthird" {
		t.Fatalf("duplicate = %q", got)
	}
	if _, err := driver.ShortcutChord(primary(), 'l'); err != nil {
		t.Fatal(err)
	}
	if got := state.space.Current().Editor.String(); got != "first\nsecond\nthird" {
		t.Fatalf("delete = %q", got)
	}
}

func TestEncodingMenuChangesSavedLineEndings(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("first\nsecond"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(4))
	clickID(t, driver, menuItemScope.At(1))
	if state.space.Current().LineEnding() != document.CRLF || !state.space.Current().Dirty() {
		t.Fatal("encoding menu did not change the document format")
	}
	path := filepath.Join(t.TempDir(), "format.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveID)
	if state.space.Current().Dirty() {
		t.Fatal("format remained dirty after save")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first\r\nsecond" {
		t.Fatalf("format saved as %q, err=%v", data, err)
	}
}

func TestEditMenuIndentAndUnindent(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("one\ntwo"); err != nil {
		t.Fatal(err)
	}
	state.space.Current().Editor.SetSelection(text.Range{Start: 0, End: 7})
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(8))
	if got := state.space.Current().Editor.String(); got != "\tone\n\ttwo" {
		t.Fatalf("indent menu = %q", got)
	}
	state.space.Current().Editor.SetSelection(text.Range{Start: 0, End: 9})
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(9))
	if got := state.space.Current().Editor.String(); got != "one\ntwo" {
		t.Fatalf("unindent menu = %q", got)
	}
}

func TestViewMenuTogglesLineNumbersAndZoom(t *testing.T) {
	state, driver := startDriver(t)
	hasLineNumber := func() bool {
		for _, label := range driver.Snapshot().Labels {
			if label.Value == "1" && label.Bounds.Y >= menuHeight+toolbarHeight+tabHeight {
				return true
			}
		}
		return false
	}
	if !hasLineNumber() {
		t.Fatal("initial editor should have a gutter")
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(0))
	if hasLineNumber() {
		t.Fatal("line-number toggle left the gutter visible")
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(2))
	if state.fontSize != 14 {
		t.Fatalf("zoom in font size = %d", state.fontSize)
	}
}

func TestSettingsMenuChangesCodeEditingAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	clickID(t, driver, menuScope.At(5))
	clickID(t, driver, menuItemScope.At(1))
	for _, index := range []int{3, 5, 7, 8} {
		clickID(t, driver, menuScope.At(6))
		clickID(t, driver, menuItemScope.At(index))
	}
	options, ok := state.space.Current().Editor.Code()
	if !ok || options.TabWidth != 8 || !options.HardTabs || options.AutoIndent || options.AutoClose {
		t.Fatalf("editor options = %+v, set=%v", options, ok)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("("); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	if got := state.space.Current().Editor.String(); got != "(\t" {
		t.Fatalf("configured code input = %q", got)
	}
	stored, err := preferences.Load(filepath.Join(filepath.Dir(path), "settings.json"))
	if err != nil || stored.TabWidth != 8 || stored.IndentMode != "tabs" || stored.AutoIndent || stored.AutoClose {
		t.Fatalf("saved preferences = %+v, %v", stored, err)
	}
}

func TestViewPreferencesRestoreWithExplicitFileArguments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "VivaPad", "session.json")
	file := filepath.Join(root, "sample.txt")
	if err := os.WriteFile(file, []byte("sample"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, path)
	state.command("Show Line Numbers")
	state.command("Word Wrap")
	state.command("Zoom In")
	restored := newStartupModel([]string{file}, path)
	if restored.showLineNumbers || !restored.wrap || restored.fontSize != 14 || restored.space.Current().Path == "" {
		t.Fatalf("restored view: lines=%v wrap=%v size=%d file=%q", restored.showLineNumbers, restored.wrap, restored.fontSize, restored.space.Current().Path)
	}
}

func TestKeyboardZoomPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	clickID(t, driver, editorID)
	if handled, err := driver.ShortcutChord(primary(), '+'); err != nil || !handled {
		t.Fatalf("zoom shortcut handled=%v, err=%v", handled, err)
	}
	if got := newStartupModel(nil, path).fontSize; got != 14 {
		t.Fatalf("restarted font size after keyboard zoom = %d", got)
	}
}

func TestDamagedSettingsLeaveDefaultsAndReportError(t *testing.T) {
	root := filepath.Join(t.TempDir(), "VivaPad")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, filepath.Join(root, "session.json"))
	if state.fontSize != 13 || !state.showLineNumbers || state.wrap || !strings.Contains(state.notice, "Settings") {
		t.Fatalf("bad settings fallback: size=%d lines=%v wrap=%v notice=%q", state.fontSize, state.showLineNumbers, state.wrap, state.notice)
	}
}

func TestSplitViewSharesDocumentButKeepsTwoEditors(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("left"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	if !state.splitEnabled || state.paneDocs[0] != state.paneDocs[1] {
		t.Fatal("single-document split did not clone its view")
	}
	clickID(t, driver, rightEditorID)
	if err := driver.Type(" right"); err != nil {
		t.Fatal(err)
	}
	if state.space.Current().Editor.String() != "left right" || state.activePane != 1 ||
		state.codeEditorForPane(0) == state.codeEditorForPane(1) {
		t.Fatalf("split state = text:%q active:%d", state.space.Current().Editor.String(), state.activePane)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	if state.splitEnabled {
		t.Fatal("split toggle did not return to one editor")
	}
}

func TestSplitViewRoutesEditingToSelectedDocument(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("alpha"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, newID)
	clickID(t, driver, editorID)
	if err := driver.Type("beta"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	if state.paneDocs[0].Name != "new 1" || state.paneDocs[1].Name != "new 2" {
		t.Fatalf("split documents = %q / %q", state.paneDocs[0].Name, state.paneDocs[1].Name)
	}
	clickID(t, driver, leftPaneID)
	if state.activePane != 0 || state.space.Current().Name != "new 1" {
		t.Fatal("left pane did not become active")
	}
	clickID(t, driver, rightEditorID)
	if err := driver.Type("!"); err != nil {
		t.Fatal(err)
	}
	if state.space.Current().Name != "new 2" || state.paneDocs[0].Editor.String() != "alpha" || state.paneDocs[1].Editor.String() != "beta!" {
		t.Fatalf("pane edits = left:%q right:%q", state.paneDocs[0].Editor.String(), state.paneDocs[1].Editor.String())
	}
}

func TestStatusPositionFollowsActiveEditor(t *testing.T) {
	state, driver := startDriver(t)
	assertPosition := func(want string) {
		t.Helper()
		for _, label := range driver.Snapshot().Texts {
			if label.Value == want {
				return
			}
		}
		t.Fatalf("status does not show %q for pane %d", want, state.activePane)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("first\n\u732b\U0001f600"); err != nil {
		t.Fatal(err)
	}
	assertPosition("Ln: 2  Col: 3")
	clickID(t, driver, newID)
	clickID(t, driver, editorID)
	if err := driver.Type("a\n\nb"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	clickID(t, driver, rightEditorID)
	assertPosition("Ln: 3  Col: 2")
	clickID(t, driver, leftPaneID)
	assertPosition("Ln: 2  Col: 3")
}

func TestSplitDividerClampsAtMinimumWidthAndMovesByKey(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	var divider gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == splitterID {
			divider = hit.Bounds
		}
	}
	if divider.Width == 0 {
		t.Fatal("split divider has no hit region")
	}
	startX, centerY := divider.X+divider.Width/2, divider.Y+divider.Height/2
	if err := driver.Press(startX, centerY, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(10, centerY); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(10, centerY, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if got := state.splitPosition(640); got < 200 || got > 432 {
		t.Fatalf("split drag left pane width = %g", got)
	}
	before := state.splitPosition(640)
	if err := driver.Key(gfx.KeyRight, 0); err != nil {
		t.Fatal(err)
	}
	if got := state.splitPosition(640); got <= before {
		t.Fatalf("keyboard splitter did not move: before=%g after=%g", before, got)
	}
}

func TestSplitEditorsScrollIndependently(t *testing.T) {
	state := newModel()
	state.space.Current().Editor.Insert(strings.Repeat("left line\n", 120))
	state.space.Current().Editor.MoveTo(0, false)
	state.space.NewDocument().Editor.Insert(strings.Repeat("right line\n", 120))
	state.space.Current().Editor.MoveTo(0, false)
	state.toggleSplit()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	firstLeft, _ := state.codeEditorForPane(0).VisibleLines()
	firstRight, _ := state.codeEditorForPane(1).VisibleLines()
	if firstLeft != 0 || firstRight != 0 {
		t.Fatalf("initial visible lines = %d / %d", firstLeft, firstRight)
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: 100, Y: 300, DeltaY: 240}); err != nil {
		t.Fatal(err)
	}
	scrolledLeft, _ := state.codeEditorForPane(0).VisibleLines()
	scrolledRight, _ := state.codeEditorForPane(1).VisibleLines()
	if scrolledLeft == 0 || scrolledRight != 0 {
		t.Fatalf("left scroll affected views = %d / %d", scrolledLeft, scrolledRight)
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: 1000, Y: 300, DeltaY: 240}); err != nil {
		t.Fatal(err)
	}
	if currentLeft, _ := state.codeEditorForPane(0).VisibleLines(); currentLeft != scrolledLeft {
		t.Fatalf("right scroll moved left view from %d to %d", scrolledLeft, currentLeft)
	}
	if currentRight, _ := state.codeEditorForPane(1).VisibleLines(); currentRight == 0 {
		t.Fatal("right view did not scroll")
	}
}

func TestClosingSplitDocumentRebindsBothViews(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, newID)
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	closed := state.paneDocs[1]
	clickID(t, driver, rightCloseScope.At(state.tabHitIndex(state.space.Documents[1])))
	if len(state.space.Documents) != 1 || state.paneDocs[0] != state.space.Current() || state.paneDocs[1] != state.space.Current() || state.secondEditors[closed] != nil {
		t.Fatalf("split close retained document: %+v", state.paneDocs)
	}
}

func TestSplitSearchResultOpensInActivePane(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "result.txt")
	if err := os.WriteFile(path, []byte("needle"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	state.toggleSplit()
	state.fileResults = searchfiles.Result{Matches: []searchfiles.Match{{Path: path, Source: "needle", Start: 0, End: 6, Text: "needle"}}}
	state.resultsOpen = true
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	left := state.paneDocs[0]
	clickID(t, driver, rightPaneID)
	clickID(t, driver, resultScope.At(0))
	if state.paneDocs[0] != left || state.paneDocs[1].Name != "result.txt" || state.activePane != 1 {
		t.Fatalf("result pane assignment = left:%p right:%q active:%d", state.paneDocs[0], state.paneDocs[1].Name, state.activePane)
	}
}

func TestSplitPaneRebindsAfterExternalReplacementReload(t *testing.T) {
	root := t.TempDir()
	leftPath, rightPath := filepath.Join(root, "left.txt"), filepath.Join(root, "right.txt")
	for _, path := range []string{leftPath, rightPath} {
		if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{leftPath, rightPath})
	state.toggleSplit()
	left, oldRight := state.paneDocs[0], state.paneDocs[1]
	if err := os.WriteFile(rightPath, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(rightPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.refreshReplacedTabs(searchfiles.ApplyResult{Paths: []string{canonical}}); err != nil {
		t.Fatal(err)
	}
	if state.paneDocs[0] != left || state.paneDocs[1] == oldRight || state.paneDocs[1].Editor.String() != "new" {
		t.Fatalf("reload pane pointers = %+v", state.paneDocs)
	}
}

func TestSplitViewHasIndependentTabStrips(t *testing.T) {
	state := newModel()
	state.space.NewDocument()
	state.space.NewDocument()
	state.toggleSplit()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	hits := map[gfx.ID]bool{}
	for _, hit := range driver.Snapshot().Hits {
		hits[hit.ID] = true
	}
	if !hits[leftTabScope.At(state.tabHitIndex(state.space.Documents[0]))] || !hits[leftTabScope.At(state.tabHitIndex(state.space.Documents[1]))] || !hits[rightTabScope.At(state.tabHitIndex(state.space.Documents[2]))] || hits[tabScope.At(state.tabHitIndex(state.space.Documents[2]))] {
		t.Fatalf("pane tab ownership = %+v", hits)
	}
	clickID(t, driver, leftTabScope.At(state.tabHitIndex(state.space.Documents[0])))
	if state.activePane != 0 || state.paneDocs[0].Name != "new 1" || state.paneDocs[1].Name != "new 3" {
		t.Fatalf("selected panes = %+v, active=%d", state.paneDocs, state.activePane)
	}
}

func TestNewDocumentAppearsOnlyInActiveSplitView(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, newID)
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	clickID(t, driver, newID)
	if state.paneOwner[state.space.Current()] != 1 {
		t.Fatalf("new tab owner = %d", state.paneOwner[state.space.Current()])
	}
	clickID(t, driver, rightTabScope.At(state.tabHitIndex(state.space.Documents[2])))
	clickID(t, driver, leftPaneID)
	clickID(t, driver, newID)
	if state.paneOwner[state.space.Current()] != 0 {
		t.Fatalf("left new tab owner = %d", state.paneOwner[state.space.Current()])
	}
}

func TestClosingClonedViewKeepsTheDirtyDocument(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("keep this draft"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	original := state.space.Current()
	clickID(t, driver, rightCloseScope.At(state.tabHitIndex(state.space.Documents[0])))
	if state.splitEnabled || len(state.space.Documents) != 1 || state.space.Current() != original || !original.Dirty() || state.pendingClose >= 0 {
		t.Fatalf("clone close discarded shared document: %+v", state.space)
	}
}

func TestRightTabOverflowCanReachEarlierDocuments(t *testing.T) {
	state := newModel()
	state.showInActivePane(state.space.NewDocument())
	state.toggleSplit()
	for range 8 {
		state.showInActivePane(state.space.NewDocument())
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	for range 10 {
		clickID(t, driver, rightTabsBackID)
	}
	clickID(t, driver, rightTabScope.At(state.tabHitIndex(state.space.Documents[1])))
	if state.activePane != 1 || state.space.Current().Name != "new 2" {
		t.Fatalf("right tab navigation = active:%d name:%q", state.activePane, state.space.Current().Name)
	}
}

func TestMoveDocumentToOtherViewKeepsTheSourceDocument(t *testing.T) {
	state := newModel()
	state.showInActivePane(state.space.NewDocument())
	state.showInActivePane(state.space.NewDocument())
	state.toggleSplit()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, leftTabScope.At(state.tabHitIndex(state.space.Documents[1])))
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(6))
	if !state.splitEnabled || state.activePane != 1 || state.paneDocs[0].Name != "new 1" || state.paneDocs[1].Name != "new 2" || state.paneOwner[state.paneDocs[1]] != 1 {
		t.Fatalf("move to other view = %+v active=%d", state.paneDocs, state.activePane)
	}
}

func TestCloneDocumentToOtherViewSharesItsBuffer(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("shared"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(7))
	if !state.splitEnabled || state.activePane != 1 || state.paneDocs[0] != state.paneDocs[1] || state.paneOwner[state.paneDocs[0]] != 0 {
		t.Fatalf("clone to other view = %+v", state.paneDocs)
	}
	clickID(t, driver, rightEditorID)
	if err := driver.Type(" buffer"); err != nil {
		t.Fatal(err)
	}
	if got := state.paneDocs[0].Editor.String(); got != "shared buffer" {
		t.Fatalf("clone did not share buffer: %q", got)
	}
}

func TestControlTabCyclesOnlyActiveView(t *testing.T) {
	state := newModel()
	state.showInActivePane(state.space.NewDocument())
	state.showInActivePane(state.space.NewDocument())
	state.toggleSplit()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, leftTabScope.At(state.tabHitIndex(state.space.Documents[0])))
	if err := driver.Key(gfx.KeyTab, gfx.ModifierControl); err != nil {
		t.Fatal(err)
	}
	if state.space.Current().Name != "new 2" || state.paneDocs[1].Name != "new 3" {
		t.Fatalf("forward tab cycle = active:%q right:%q", state.space.Current().Name, state.paneDocs[1].Name)
	}
	if err := driver.Key(gfx.KeyTab, gfx.ModifierControl|gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if state.space.Current().Name != "new 1" || state.activePane != 0 {
		t.Fatalf("backward tab cycle = %q pane=%d", state.space.Current().Name, state.activePane)
	}
}

func TestSaveAllWritesEveryDirtyTab(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("first"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, newID)
	clickID(t, driver, editorID)
	if err := driver.Type("second"); err != nil {
		t.Fatal(err)
	}
	firstPath := filepath.Join(t.TempDir(), "first.txt")
	secondPath := filepath.Join(t.TempDir(), "second.txt")
	if err := driver.QueueSaveFile(firstPath, nil); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueSaveFile(secondPath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(5))
	first, firstErr := os.ReadFile(firstPath)
	second, secondErr := os.ReadFile(secondPath)
	if firstErr != nil || secondErr != nil || string(first) != "first" || string(second) != "second" || state.space.Documents[0].Dirty() || state.space.Documents[1].Dirty() {
		t.Fatalf("save all = %q/%q, errors=%v/%v", first, second, firstErr, secondErr)
	}
}

func TestEncodingMenuExposesSelectedFormat(t *testing.T) {
	_, driver := startDriver(t)
	clickID(t, driver, menuScope.At(4))
	selected := map[int]bool{}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(0) || hit.ID == menuItemScope.At(1) || hit.ID == menuItemScope.At(2) || hit.ID == menuItemScope.At(3) {
			index, _ := hit.ID.Name().Index()
			if hit.Role != gfx.RoleMenuItem {
				t.Fatalf("encoding entry role = %v", hit.Role)
			}
			selected[index] = hit.Checked
		}
	}
	if !selected[0] || selected[1] || !selected[2] || selected[3] {
		t.Fatalf("selected encoding entries = %+v", selected)
	}
}

func TestEncodingMenuSavesUTF16LE(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("中"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(4))
	clickID(t, driver, menuItemScope.At(4))
	if state.space.Current().EncodingLabel() != "UTF-16 LE" {
		t.Fatalf("encoding = %q", state.space.Current().EncodingLabel())
	}
	path := filepath.Join(t.TempDir(), "utf16.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveID)
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, []byte{0xff, 0xfe, 0x2d, 0x4e}) {
		t.Fatalf("saved UTF-16 = %x, err=%v", data, err)
	}
}

func TestOpenAsGB18030RestoresSessionAndExplicitArgument(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "chinese.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte{0xd6, 0xd0, 0xce, 0xc4}, 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(7))
	if got := state.space.Current(); got.Editor.String() != "中文" || got.Encoding() != document.GB18030 {
		t.Fatalf("opened legacy document = %+v", got)
	}
	newConfig(state).OnCloseRequest(func() {})
	stored, err := session.Load(sessionPath)
	if err != nil || len(stored.Encodings) != 1 {
		t.Fatalf("stored encoding = %+v, %v", stored, err)
	}
	for _, paths := range [][]string{nil, {path}} {
		restored := newStartupModel(paths, sessionPath)
		if got := restored.space.Current(); got.Editor.String() != "中文" || got.Encoding() != document.GB18030 {
			t.Fatalf("restored legacy document with args=%v: %+v, notice=%q", paths, got, restored.notice)
		}
	}
}

func TestEncodingMenuConvertsUTF8ToGB18030(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("中文"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(4))
	clickID(t, driver, menuItemScope.At(6))
	path := filepath.Join(t.TempDir(), "chinese.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveID)
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, []byte{0xd6, 0xd0, 0xce, 0xc4}) || state.space.Current().Dirty() {
		t.Fatalf("saved conversion = %x, %v, dirty=%v", data, err, state.space.Current().Dirty())
	}
}

func TestLegacyDirtyBufferRecoversOntoItsOriginalFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "legacy.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte{0xd6, 0xd0}, 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, sessionPath)
	if err := state.space.OpenWithEncoding(path, document.GB18030); err != nil {
		t.Fatal(err)
	}
	state.space.Current().Editor.MoveTo(1, false)
	state.space.Current().Editor.Insert("文")
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, sessionPath)
	if got := restored.space.Current(); got.Path == "" || got.Editor.String() != "中文" || got.Encoding() != document.GB18030 || !got.Dirty() {
		t.Fatalf("restored legacy backup = %+v, notice=%q", got, restored.notice)
	}
}

func TestDiscardedEncodingConversionDoesNotCorruptNextSession(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "utf8.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte("中文"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.space.Current().SetEncoding(document.GB18030)
	newConfig(state).OnCloseRequest(func() {})
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, discardID)
	stored, err := session.Load(sessionPath)
	if err != nil || len(stored.Encodings) != 0 {
		t.Fatalf("discarded conversion was remembered: %+v, %v", stored, err)
	}
	if got := newStartupModel(nil, sessionPath).space.Current(); got.Editor.String() != "中文" || got.Encoding() != document.UTF8 {
		t.Fatalf("next session after discard = %+v", got)
	}
}

func TestRecoveryKeepsLegacyDiskEncodingDuringUnsavedConversion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "western.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte{'c', 'a', 'f', 0xe9}, 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, sessionPath)
	if err := state.space.OpenWithEncoding(path, document.Windows1252); err != nil {
		t.Fatal(err)
	}
	state.space.Current().SetEncoding(document.UTF8)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, sessionPath)
	got := restored.space.Current()
	if got.Path == "" || got.Editor.String() != "café" || got.Encoding() != document.UTF8 || got.SavedEncoding() != document.Windows1252 || !got.Dirty() {
		t.Fatalf("converted backup = %+v, notice=%q", got, restored.notice)
	}
	if err := got.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "café" {
		t.Fatalf("saved converted recovery = %x, %v", data, err)
	}
}

func TestSaveAsCannotOverwriteAnotherOpenTab(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.txt")
	secondPath := filepath.Join(root, "second.txt")
	if err := os.WriteFile(firstPath, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{firstPath, secondPath})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.space.Select(0)
	if err := driver.QueueSaveFile(secondPath, nil); err != nil {
		t.Fatal(err)
	}
	state.save(0, true)
	output, err := os.ReadFile(secondPath)
	if err != nil || string(output) != "second" || state.space.Documents[0].Path == state.space.Documents[1].Path {
		t.Fatalf("overwrote open tab: %q, err=%v", output, err)
	}
}

func TestReloadFromDiskKeepsSplitViewAndLanguage(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{first, second})
	left, old := state.space.Documents[0], state.space.Documents[1]
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, old}
	state.paneOwner[old] = 1
	state.activePane = 1
	state.languages[old] = ".go"
	if err := os.WriteFile(second, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(9))
	current := state.space.Current()
	if current == old || current.Editor.String() != "after" || current.Dirty() || state.paneDocs[0] != left || state.paneDocs[1] != current || state.paneOwner[current] != 1 || state.languageFor(current) != ".go" {
		t.Fatalf("reloaded split tab = %+v pane=%+v owner=%d language=%q", current, state.paneDocs, state.paneOwner[current], state.languageFor(current))
	}
}

func TestDirtyReloadRequiresConfirmationAndCancelKeepsEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reload.txt")
	if err := os.WriteFile(path, []byte("disk"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	original := state.space.Current()
	original.Editor.Insert(" local")
	if err := os.WriteFile(path, []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(9))
	if state.pendingReload != 0 || state.space.Current() != original {
		t.Fatalf("dirty reload bypassed dialog: %+v pending=%d", state.space.Current(), state.pendingReload)
	}
	clickID(t, driver, newID)
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	if len(state.space.Documents) != 1 || allowed || state.pendingReload != 0 {
		t.Fatal("reload confirmation allowed underlying action")
	}
	clickID(t, driver, reloadCancelID)
	if state.pendingReload != -1 || state.space.Current() != original || !original.Dirty() || original.Editor.String() != " localdisk" {
		t.Fatalf("cancel discarded local edit: %+v pending=%d", state.space.Current(), state.pendingReload)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(9))
	clickID(t, driver, reloadConfirmID)
	if state.pendingReload != -1 || state.space.Current() == original || state.space.Current().Editor.String() != "external" || state.space.Current().Dirty() {
		t.Fatalf("confirmed reload = %+v pending=%d", state.space.Current(), state.pendingReload)
	}
}

func TestFailedConfirmedReloadKeepsDirtyDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.txt")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	original := state.space.Current()
	original.Editor.Insert("local ")
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(9))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, reloadConfirmID)
	if state.pendingReload != -1 || state.space.Current() != original || !original.Dirty() || !strings.Contains(state.notice, "Reload failed") {
		t.Fatalf("failed reload lost edits: %+v pending=%d notice=%q", state.space.Current(), state.pendingReload, state.notice)
	}
}

func TestUntitledReloadIsRejectedAndEscapeCancelsDirtyReload(t *testing.T) {
	state, driver := startDriver(t)
	state.command("Reload from Disk")
	if state.pendingReload != -1 || !strings.Contains(state.notice, "Save the document") {
		t.Fatalf("untitled reload = pending:%d notice:%q", state.pendingReload, state.notice)
	}
	path := filepath.Join(t.TempDir(), "reload.txt")
	if err := os.WriteFile(path, []byte("disk"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	original := state.space.Current()
	original.Editor.Insert("local ")
	state.command("Reload from Disk")
	if state.pendingReload < 0 {
		t.Fatal("dirty reload did not ask")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.pendingReload != -1 || state.space.Current() != original || !original.Dirty() {
		t.Fatalf("Esc lost dirty document: %+v pending=%d", state.space.Current(), state.pendingReload)
	}
}

func TestGoFileSupportsCommentToggleAndTabIndent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.space.Current().Editor.MoveTo(0, false)
	clickID(t, driver, editorID)
	state.space.Current().Editor.MoveTo(0, false)
	if _, err := driver.ShortcutChord(primary(), '/'); err != nil {
		t.Fatal(err)
	}
	if got := state.space.Current().Editor.String(); got != "// package main\n" {
		t.Fatalf("comment toggle = %q", got)
	}
	state.space.Current().Editor.MoveTo(0, false)
	if err := driver.Key(gfx.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	if got := state.space.Current().Editor.String(); got != "    // package main\n" {
		t.Fatalf("tab indent = %q", got)
	}
}

func TestGoFileAutoClosesAndIndentsBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "block.go")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, editorID)
	if err := driver.Type("{"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if got := state.space.Current().Editor.String(); got != "{\n    \n}" {
		t.Fatalf("code editing = %q", got)
	}
}

func TestSearchPanelMarksEveryVisibleMatch(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("cat cat"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchID)
	clickID(t, driver, queryID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	marks := state.searchMarks()
	if len(marks) != 2 || marks[0].Start != 0 || marks[1].Start != 4 {
		t.Fatalf("search marks = %+v", marks)
	}
}

func TestClosingTabReleasesItsCodeEditor(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, newID)
	if len(state.editors) != 2 {
		t.Fatalf("editor count after new tab = %d", len(state.editors))
	}
	clickID(t, driver, tabCloseScope.At(state.tabHitIndex(state.space.Documents[1])))
	if len(state.editors) != 1 {
		t.Fatalf("closed tab retained an editor: %d", len(state.editors))
	}
}

func TestWordWrapSwitchesHorizontalScrolling(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type(strings.Repeat("longword ", 140)); err != nil {
		t.Fatal(err)
	}
	findHorizontalBar := func() bool {
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == horizontalBarID {
				return true
			}
		}
		return false
	}
	if !findHorizontalBar() {
		t.Fatal("unwrapped editor has no horizontal scrollbar")
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(1))
	if !state.wrap || findHorizontalBar() {
		t.Fatalf("word wrap=%v, horizontal scrollbar=%v", state.wrap, findHorizontalBar())
	}
	if caret, ok := state.codeEditor().CaretBounds(); !ok || caret.Y <= menuHeight+toolbarHeight+tabHeight+20 {
		t.Fatalf("wrapped caret = %+v, visible=%v", caret, ok)
	}
}

func TestLanguageMenuOverridesSyntaxPerTab(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(5))
	clickID(t, driver, menuItemScope.At(1))
	if got := state.languageFor(state.space.Current()); got != ".go" {
		t.Fatalf("language override = %q", got)
	}
	if options, ok := state.space.Current().Editor.Code(); !ok || options.LineComment != "//" {
		t.Fatalf("Go editing options = %+v, %v", options, ok)
	}
	clickID(t, driver, newID)
	if got := state.languageFor(state.space.Current()); got != "" {
		t.Fatalf("new tab inherited language %q", got)
	}
	clickID(t, driver, tabScope.At(state.tabHitIndex(state.space.Documents[0])))
	if got := state.languageFor(state.space.Current()); got != ".go" {
		t.Fatalf("original tab lost language %q", got)
	}
}

func TestFilesDialogPassesPrintableKeysToItsFields(t *testing.T) {
	for _, command := range []string{"Find in Files...", "Replace in Files..."} {
		t.Run(command, func(t *testing.T) {
			state, driver := startDriver(t)
			clickID(t, driver, menuScope.At(2))
			clickID(t, driver, searchCommandID(t, command))
			fields := []gfx.ID{queryID, filesRootID, filesPatternID}
			if state.filesReplaceMode {
				fields = append(fields, replacementID)
			}
			for _, field := range fields {
				clickID(t, driver, field)
				for _, modifiers := range []gfx.Modifiers{0, gfx.ModifierShift} {
					if handled, err := driver.ShortcutChord(modifiers, 'a'); err != nil || handled {
						t.Fatalf("field %v swallowed a printable key: handled=%v, error=%v", field, handled, err)
					}
				}
				if handled, err := driver.ShortcutChord(primary(), 'n'); err != nil || !handled || len(state.space.Documents) != 1 {
					t.Fatalf("New escaped the files dialog: handled=%v, error=%v", handled, err)
				}
			}
		})
	}
}

func TestFindInFilesOpensExactResult(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.go")
	if err := os.WriteFile(path, []byte("first\n猫 needle here\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.txt"), []byte("needle"), 0644); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find in Files..."))
	if !state.filesDialogOpen {
		t.Fatal("find-in-files dialog did not open")
	}
	if err := driver.QueueOpenDirectory(root, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, filesBrowseID)
	clickID(t, driver, queryID)
	if err := driver.Type("needle"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, filesPatternID)
	if err := driver.SelectText(0, state.filesPattern.RuneCount()); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("*.go"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, filesSearchID)
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.filesSearching || !state.resultsOpen || len(state.fileResults.Matches) != 1 {
		t.Fatalf("find-in-files result = searching:%v open:%v %+v", state.filesSearching, state.resultsOpen, state.fileResults)
	}
	clickID(t, driver, resultScope.At(0))
	doc := state.space.Current()
	selection := doc.Editor.Selection()
	if doc.Name != "example.go" || string([]rune(doc.Editor.String())[selection.Start:selection.End]) != "needle" {
		t.Fatalf("opened result = %q selection=%+v", doc.Name, selection)
	}
}

func TestFindInFilesOpensLegacyResultInSelectedEncoding(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "legacy.txt")
	if err := os.WriteFile(path, []byte{0xd6, 0xd0, 0xce, 0xc4, ' ', 0xd6, 0xd0}, 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find in Files..."))
	if err := driver.QueueOpenDirectory(root, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, filesBrowseID)
	clickID(t, driver, queryID)
	if err := driver.Type("中文"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, filesGB18030ID)
	clickID(t, driver, filesSearchID)
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(state.fileResults.Matches) != 1 || state.fileResults.Matches[0].Encoding != document.GB18030 {
		t.Fatalf("legacy results = %+v, notice=%q", state.fileResults, state.notice)
	}
	clickID(t, driver, resultScope.At(0))
	doc := state.space.Current()
	selection := doc.Editor.Selection()
	if doc.Encoding() != document.GB18030 || string([]rune(doc.Editor.String())[selection.Start:selection.End]) != "中文" {
		t.Fatalf("opened result = %+v selection=%+v", doc, selection)
	}
}

func TestFindInFilesShortcutAndInvalidRegex(t *testing.T) {
	state, driver := startDriver(t)
	if _, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'f'); err != nil {
		t.Fatal(err)
	}
	if !state.filesDialogOpen {
		t.Fatal("primary+shift+F did not open Find in Files")
	}
	clickID(t, driver, queryID)
	if err := driver.Type("("); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, regexID)
	clickID(t, driver, filesSearchID)
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.filesSearching || !state.filesDialogOpen || !strings.Contains(state.notice, "missing") {
		t.Fatalf("invalid regex state = searching:%v dialog:%v notice:%q", state.filesSearching, state.filesDialogOpen, state.notice)
	}
	visibleError := false
	for _, label := range driver.Snapshot().Labels {
		visibleError = visibleError || strings.Contains(label.Value, "missing") && label.Bounds.Y < 550
	}
	if !visibleError {
		t.Fatal("directory search error is hidden behind the modal")
	}
}

func TestFindInFilesLeavesControlSpaceToFocusedInputMethod(t *testing.T) {
	state := newModel()
	state.openFilesDialog()
	onShortcut := newConfig(state).OnShortcut
	for _, tc := range []struct {
		name    string
		event   gfx.KeyEvent
		handled bool
	}{
		{"normalized search field", gfx.KeyEvent{Target: queryID, Text: " ", Modifiers: gfx.ModifierControl}, false},
		{"space key in directory field", gfx.KeyEvent{Target: filesRootID, Key: gfx.KeySpace, Modifiers: gfx.ModifierControl}, false},
		{"hidden replacement field", gfx.KeyEvent{Target: replacementID, Text: " ", Modifiers: gfx.ModifierControl}, true},
		{"background editor", gfx.KeyEvent{Target: editorID, Text: " ", Modifiers: gfx.ModifierControl}, true},
		{"global shortcut", gfx.KeyEvent{Target: queryID, Text: "s", Modifiers: primary()}, true},
		{"other space chord", gfx.KeyEvent{Target: queryID, Text: " ", Modifiers: gfx.ModifierControl | gfx.ModifierShift}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := onShortcut(tc.event).Handled; got != tc.handled {
				t.Fatalf("shortcut handled = %v, want %v", got, tc.handled)
			}
		})
	}
}

func TestFindInFilesEnterStartsSearch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "match.txt"), []byte("needle"), 0644); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Find in Files..."))
	if err := driver.QueueOpenDirectory(root, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, filesBrowseID)
	clickID(t, driver, queryID)
	if err := driver.Type("needle"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.resultsOpen || len(state.fileResults.Matches) != 1 {
		t.Fatalf("Enter did not show results: %+v", state.fileResults)
	}
}

func TestFindPanelEnterFindsNext(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("first second first"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, searchID)
	clickID(t, driver, queryID)
	if err := driver.Type("second"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if selection := state.space.Current().Editor.Selection(); selection.Start != 6 || selection.End != 12 {
		t.Fatalf("Enter search selection = %+v", selection)
	}
}

func TestReplaceInFilesPreviewThenConfirmRefreshesOpenTab(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "replace.txt")
	if err := os.WriteFile(path, []byte("cat CAT"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	previous := state.space.Current()
	previous.Editor.MoveTo(4, false)
	state.tabColors[previous] = 3
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Replace in Files..."))
	if !state.filesDialogOpen || !state.filesReplaceMode {
		t.Fatal("Replace in Files dialog did not open")
	}
	clickID(t, driver, queryID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacementID)
	if err := driver.Type("dog"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacePreviewID)
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.replaceConfirmOpen || state.replacePlan == nil || state.replacePlan.Replacements != 2 {
		t.Fatalf("missing replacement preview: %+v", state.replacePlan)
	}
	if data, _ := os.ReadFile(path); string(data) != "cat CAT" {
		t.Fatalf("preview modified disk: %q", data)
	}
	clickID(t, driver, replaceConfirmID)
	for attempts := 0; attempts < 100 && state.filesApplying; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "dog dog" || state.space.Current().Editor.String() != "dog dog" || state.replaceConfirmOpen {
		t.Fatalf("confirmed replacement = disk:%q editor:%q modal:%v", data, state.space.Current().Editor.String(), state.replaceConfirmOpen)
	}
	current := state.space.Current()
	if current == previous || current.Dirty() || current.Editor.CanUndo() {
		t.Fatal("confirmed replacement did not start a fresh, clean history")
	}
	if current.Editor.Caret() != 4 || state.tabColors[current] != 3 {
		t.Errorf("replacement reset tab state: caret=%d color=%d", current.Editor.Caret(), state.tabColors[current])
	}
	requireEditorCommandFocus(t, driver, editorID)
}

func TestReplaceInFilesLegacyChoiceRefreshesMatchingOpenTab(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "legacy.txt")
	if err := os.WriteFile(path, []byte{0xd6, 0xd0, ' ', 0xd6, 0xd0}, 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.space.OpenWithEncoding(path, document.GB18030); err != nil {
		t.Fatal(err)
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Replace in Files..."))
	clickID(t, driver, queryID)
	if err := driver.Type("中"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacementID)
	if err := driver.Type("文"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, filesGB18030ID)
	clickID(t, driver, replacePreviewID)
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.replaceConfirmOpen || state.replacePlan == nil || state.replacePlan.Replacements != 2 {
		t.Fatalf("legacy preview = %+v notice=%q", state.replacePlan, state.notice)
	}
	clickID(t, driver, replaceConfirmID)
	for attempts := 0; attempts < 100 && state.filesApplying; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, []byte{0xce, 0xc4, ' ', 0xce, 0xc4}) || state.space.Current().Editor.String() != "文 文" {
		t.Fatalf("legacy apply = disk:%x editor:%q err:%v notice:%q", data, state.space.Current().Editor.String(), err, state.notice)
	}
}

func TestReplaceInFilesRejectsOpenTabWithDifferentEncoding(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ascii.txt")
	if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Replace in Files..."))
	clickID(t, driver, queryID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacementID)
	if err := driver.Type("中文"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, filesGB18030ID)
	clickID(t, driver, replacePreviewID)
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.replaceConfirmOpen {
		t.Fatalf("missing preview: %q", state.notice)
	}
	clickID(t, driver, replaceConfirmID)
	if data, err := os.ReadFile(path); err != nil || string(data) != "cat" || !strings.Contains(state.notice, "another encoding") {
		t.Fatalf("mismatched tab was replaced: %q, %v, notice=%q", data, err, state.notice)
	}
	visible := false
	for _, label := range driver.Snapshot().Labels {
		visible = visible || strings.Contains(label.Value, "another encoding") && label.Bounds.Y >= 240 && label.Bounds.Y < 560
	}
	if !visible {
		t.Fatal("encoding mismatch is hidden behind the confirmation dialog")
	}
}

func TestReplaceInFilesCancelLeavesDiskUntouched(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cancel.txt")
	if err := os.WriteFile(path, []byte("needle"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Replace in Files..."))
	clickID(t, driver, queryID)
	if err := driver.Type("needle"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacementID)
	if err := driver.Type("changed"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacePreviewID)
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replaceCancelID)
	if data, _ := os.ReadFile(path); string(data) != "needle" || state.replaceConfirmOpen {
		t.Fatalf("canceled replacement = %q, modal:%v", data, state.replaceConfirmOpen)
	}
	requireEditorCommandFocus(t, driver, editorID)
}

func TestReplaceInFilesEscapeReturnsFocusToActiveRightEditor(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "right.txt")
	if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	right := state.space.Current()
	left := state.space.NewDocument()
	left.Editor.Insert("unrelated")
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	plan, err := searchfiles.PreviewReplace(context.Background(), root, "cat", "dog", document.SearchOptions{}, searchfiles.Options{})
	if err != nil || len(plan.Changes) != 1 {
		t.Fatalf("replacement preview: %+v, %v", plan, err)
	}
	state.replacePlan, state.replaceConfirmOpen = plan, true
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.replaceConfirmOpen || state.replacePlan != nil || state.space.Current() != right {
		t.Fatal("Escape did not dismiss the preview and keep the active tab")
	}
	requireEditorCommandFocus(t, driver, rightEditorID)
	if err := driver.Type("x"); err != nil || !strings.Contains(right.Editor.String(), "x") || left.Editor.String() != "unrelated" {
		t.Fatalf("first input after Escape: left=%q right=%q error=%v", left.Editor.String(), right.Editor.String(), err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "cat" {
		t.Fatalf("Escape modified disk: %q, %v", data, err)
	}
}

func TestReplaceInFilesRefusesDirtyOpenTab(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "dirty.txt")
	if err := os.WriteFile(path, []byte("cat"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.space.Current().Editor.Insert(" local")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Replace in Files..."))
	clickID(t, driver, queryID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacementID)
	if err := driver.Type("dog"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacePreviewID)
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replaceConfirmID)
	if data, _ := os.ReadFile(path); string(data) != "cat" || !state.replaceConfirmOpen || !strings.Contains(state.notice, "modified tabs") {
		t.Fatalf("dirty tab replace = disk:%q confirm:%v notice:%q", data, state.replaceConfirmOpen, state.notice)
	}
}

func TestReplaceInFilesRefusesStalePreview(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "stale.txt")
	if err := os.WriteFile(path, []byte("cat"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	revealSearchMenuItem(t, driver, searchCommandIndex(t, "Replace in Files..."))
	clickID(t, driver, searchCommandID(t, "Replace in Files..."))
	clickID(t, driver, queryID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacementID)
	if err := driver.Type("dog"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacePreviewID)
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("external cat"), 0644); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replaceConfirmID)
	for attempts := 0; attempts < 100 && state.filesApplying; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "external cat" || !strings.Contains(state.notice, "changed after preview") {
		t.Fatalf("stale preview = disk:%q notice:%q", data, state.notice)
	}
	requireEditorCommandFocus(t, driver, editorID)
	for _, label := range driver.Snapshot().Texts {
		if strings.Contains(label.Value, "changed after preview; no files written") {
			return
		}
	}
	t.Fatalf("compact status hid the stale-preview refusal: %q", state.notice)
}

func TestReplaceInFilesEnterOpensPreviewWithoutWriting(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "enter.txt")
	if err := os.WriteFile(path, []byte("cat"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Replace in Files..."))
	clickID(t, driver, queryID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacementID)
	if err := driver.Type("dog"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	for attempts := 0; attempts < 100 && state.filesSearching; attempts++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.replaceConfirmOpen {
		t.Fatal("Enter bypassed replacement preview")
	}
	if data, _ := os.ReadFile(path); string(data) != "cat" {
		t.Fatalf("Enter preview wrote disk: %q", data)
	}
}

func TestReplaceReportsTabReloadFailureAfterDiskWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	generation := state.searchReplies.advance()
	state.filesApplying = true
	state.replaceConfirmOpen = true
	state.sendFileReply(fileSearchResponse{
		kind: fileJobReplaceApply, generation: generation,
		applied: searchfiles.ApplyResult{FilesChanged: 1, Replacements: 1, Paths: []string{state.space.Current().Path}},
	})
	state.consumeFileSearch()
	if !strings.Contains(state.notice, "reload failed") {
		t.Fatalf("successful write hid reload failure: %q", state.notice)
	}
}
