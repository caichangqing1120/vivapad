package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/recovery"
)

func TestRecoveryAfterHidingSplitKeepsLatestRightDraft(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "settings", "session.json")
	state := newStartupModel(nil, sessionPath)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	clickID(t, driver, rightPaneID)
	clickID(t, driver, newID)
	clickID(t, driver, rightEditorID)
	if err := driver.Type("right draft"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	if !state.splitEnabled || state.paneOwner[doc] != 1 {
		t.Fatal("fixture did not create a right-owned draft")
	}
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	if state.splitEnabled || state.space.Current() != doc || state.paneOwner[doc] != 1 {
		t.Fatal("fixture did not fold the split into a single view")
	}
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	if err := driver.Type(" updated"); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	stored, err := recovery.Load(state.recoveryPath)
	if err != nil || stored.Split || len(stored.Tabs) != 1 || stored.Tabs[0].Pane != 0 || stored.Tabs[0].Content != "right draft updated" {
		t.Fatalf("backup after hiding split = %+v; load error = %v", stored, err)
	}
	driver.Close()
	restored := newStartupModel(nil, sessionPath)
	if restored.splitEnabled || restored.space.Current().Editor.String() != "right draft updated" || !restored.space.Current().Dirty() {
		t.Fatal("restart did not recover the latest draft in the single view")
	}
	restartedDriver, err := app.NewDriver(newConfig(restored), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restartedDriver.Close()
	if focus := restartedDriver.Snapshot().Focused; focus != editorID {
		t.Fatalf("restarted single-pane draft has focus %v; want %v", focus, editorID)
	}
	if err := restartedDriver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if got := restored.space.Current().Editor.String(); got != "right draft updatedx" {
		t.Fatalf("first key after restart left %q", got)
	}
}

func TestRecoveryValidationFailureRemainsVisibleUntilBackupSucceeds(t *testing.T) {
	state := newStartupModel(nil, filepath.Join(t.TempDir(), "settings", "session.json"))
	doc := state.space.Current()
	doc.Editor.Insert("original draft")
	state.snapshotRecovery()
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	state.tabColors[doc] = 6 // Deliberately invalid recovery metadata.
	doc.Editor.Insert(" updated")
	state.snapshotRecovery()
	_ = state.recoveryWriter.Wait()
	state.observeRecoveryError()
	state.snapshotRecovery()
	state.observeRecoveryError()
	if !strings.Contains(state.notice, "Recovery failed: recovery tab has invalid color") {
		t.Fatalf("validation failure disappeared after a wake: %q", state.notice)
	}
	stored, err := recovery.Load(state.recoveryPath)
	if err != nil || len(stored.Tabs) != 1 || stored.Tabs[0].Content != "original draft" {
		t.Fatalf("validation failure replaced the last valid backup: %+v, %v", stored, err)
	}
	state.tabColors[doc] = 2
	state.snapshotRecovery()
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	state.observeRecoveryError()
	stored, err = recovery.Load(state.recoveryPath)
	if state.notice != "" || err != nil || len(stored.Tabs) != 1 || stored.Tabs[0].Content != "original draft updated" || stored.Tabs[0].TabColor != 2 {
		t.Fatalf("successful retry failed to replace the backup and clear the warning: notice=%q, backup=%+v, err=%v", state.notice, stored, err)
	}
}
