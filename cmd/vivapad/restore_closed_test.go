package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/recent"
)

func TestRestoreRecentClosedLegacyFileAcrossRestart(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "chinese.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte{0xd6, 0xd0}, 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, sessionPath)
	if err := state.space.OpenWithEncoding(path, document.GB18030); err != nil {
		t.Fatal(err)
	}
	state.showInActivePane(state.space.Current())
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, tabCloseScope.At(0))
	if len(state.closedHistory) != 1 || state.closedHistory[0].Encoding != document.GB18030 {
		t.Fatalf("closed legacy history = %+v", state.closedHistory)
	}
	restarted := newStartupModel(nil, sessionPath)
	if len(restarted.closedHistory) != 1 {
		t.Fatalf("restarted closed history = %+v", restarted.closedHistory)
	}
	restarted.command("Restore Recent Closed File")
	if got := restarted.space.Current(); got.Editor.String() != "中" || got.Encoding() != document.GB18030 || len(restarted.closedHistory) != 0 {
		t.Fatalf("restored legacy file = %+v history=%+v", got, restarted.closedHistory)
	}
}

func TestRestoreAfterDiscardReadsSavedDiskEncodingOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "utf8.txt")
	if err := os.WriteFile(path, []byte("中文"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	doc := state.space.Current()
	doc.SetEncoding(document.GB18030)
	doc.Editor.Insert(" local")
	clickID(t, driver, tabCloseScope.At(0))
	clickID(t, driver, discardID)
	if len(state.closedHistory) != 1 || state.closedHistory[0].Encoding != document.UTF8 {
		t.Fatalf("discarded encoding was remembered: %+v", state.closedHistory)
	}
	state.command("Restore Recent Closed File")
	if got := state.space.Current(); got.Editor.String() != "中文" || got.Encoding() != document.UTF8 || got.Dirty() {
		t.Fatalf("discard restore = %+v", got)
	}
}

func TestMissingRecentlyClosedFileKeepsCurrentTabAndHistory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "missing.txt")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, tabCloseScope.At(0))
	current := state.space.Current()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	state.command("Restore Recent Closed File")
	if state.space.Current() != current || len(state.closedHistory) != 1 || !strings.Contains(state.notice, "missing.txt") {
		t.Fatalf("missing restore changed state: current=%+v history=%+v notice=%q", state.space.Current(), state.closedHistory, state.notice)
	}
}

func TestBatchCloseOrderAndRecentSubmenuRestoreShortcut(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles(paths)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(11))
	if len(state.closedHistory) != 2 || filepath.Base(state.closedHistory[0].Path) != "second.txt" {
		t.Fatalf("batch close order = %+v", state.closedHistory)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(10))
	clickID(t, driver, recentRestoreID)
	if filepath.Base(state.space.Current().Path) != "second.txt" || len(state.closedHistory) != 1 {
		t.Fatalf("submenu restore = %+v history=%+v", state.space.Current(), state.closedHistory)
	}
	if handled, err := driver.ShortcutChord(primary()|ui.ModifierShift, 't'); err != nil || !handled {
		t.Fatalf("restore shortcut handled=%v, %v", handled, err)
	}
	if filepath.Base(state.space.Current().Path) != "first.txt" || len(state.closedHistory) != 0 {
		t.Fatalf("shortcut restore = %+v history=%+v", state.space.Current(), state.closedHistory)
	}
}

func TestClearRecentFilesAlsoClearsClosedHistory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "closed.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, sessionPath)
	state.closeTab(0)
	if len(state.closedHistory) != 1 {
		t.Fatal("closed history not recorded")
	}
	state.clearRecent()
	if len(state.closedHistory) != 0 || len(newStartupModel(nil, sessionPath).closedHistory) != 0 {
		t.Fatal("Clear Recent Files left closed-file history")
	}
	if _, err := recent.Load(filepath.Join(filepath.Dir(sessionPath), "closed.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("closed history failed to clear: %v", err)
	}
}

func TestOrdinaryReopenRemovesPathFromClosedHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.txt")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, tabCloseScope.At(0))
	if len(state.closedHistory) != 1 {
		t.Fatal("close was not recorded")
	}
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, openID)
	if len(state.closedHistory) != 0 {
		t.Fatalf("reopened file still appears closed: %+v", state.closedHistory)
	}
	current := state.space.Current()
	state.command("Restore Recent Closed File")
	if state.space.Current() != current || !strings.Contains(state.notice, "No recently closed") {
		t.Fatalf("restore after ordinary reopen = current:%+v notice:%q", state.space.Current(), state.notice)
	}
}
