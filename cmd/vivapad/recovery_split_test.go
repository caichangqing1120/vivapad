package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/recovery"
	"vivapad/internal/session"
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

func TestRecoveryKeepsCaretAfterReversingSelection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.txt")
	const saved = "\u732b\r\nsaved"
	if err := os.WriteFile(path, []byte(saved), 0600); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{path}, sessionPath)
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert(" draft")
	want := text.Range{Start: 1, End: 4}
	doc.Editor.SetSelection(want)
	state.snapshotRecovery()
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	// Reversing the selection keeps its sorted bounds and changes only the caret.
	doc.Editor.SetSelection(text.Range{Start: want.End, End: want.Start})
	state.snapshotRecovery()
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, sessionPath).space.Current()
	if restored.Editor.Selection() != want || restored.Editor.Caret() != want.Start ||
		restored.Editor.String() != "\u732b\nsaved draft" || !restored.Dirty() {
		t.Fatalf("recovered selection = %+v, caret = %d, content = %q", restored.Editor.Selection(), restored.Editor.Caret(), restored.Editor.String())
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != saved {
		t.Fatalf("recovery changed disk bytes: %q, %v", data, err)
	}
}

func TestRecoveryKeepsDraftSeparateFromTabOpenedWithDifferentDecoder(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.txt")
	const saved = "caf\xc3\xa9\r\n"
	if err := os.WriteFile(path, []byte(saved), 0600); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{path}, sessionPath)
	doc := state.space.Current()
	path = doc.Path
	doc.Editor.Insert("draft ")
	const draft = "draft caf\u00e9\n"
	selection := text.Range{Start: 6, End: 10}
	doc.Editor.SetSelection(selection)
	state.snapshotRecovery()
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := session.Save(sessionPath, session.State{
		Paths: []string{path}, Active: path,
		Encodings: map[string]string{path: "Windows-1252"},
	}); err != nil {
		t.Fatal(err)
	}

	restored := newStartupModel(nil, sessionPath)
	if len(restored.space.Documents) != 2 {
		t.Fatalf("recovery attached a UTF-8 draft to another decoder: tabs=%d notice=%q", len(restored.space.Documents), restored.notice)
	}
	diskTab, draftTab := restored.space.Documents[0], restored.space.Current()
	if diskTab.Path != path || diskTab.SavedEncoding() != document.Windows1252 || diskTab.Dirty() || diskTab.Editor.String() != "caf\u00c3\u00a9\n" {
		t.Fatalf("recovery changed the existing Windows-1252 tab: %q, %s", diskTab.Editor.String(), diskTab.EncodingLabel())
	}
	if draftTab.Path != "" || !draftTab.Dirty() || draftTab.Encoding() != document.UTF8 || draftTab.Editor.String() != draft || draftTab.Editor.Selection() != selection {
		t.Fatalf("recovery did not keep an untitled UTF-8 draft: path=%q text=%q selection=%+v", draftTab.Path, draftTab.Editor.String(), draftTab.Editor.Selection())
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != saved {
		t.Fatalf("recovery changed disk bytes: %q, %v", data, err)
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

func TestRecoveryKeepsDraftsSeparateWhenSavedPathBecomesAlias(t *testing.T) {
	root := t.TempDir()
	firstPath, secondPath := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	const disk = "original\r\nlast\n"
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte(disk), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{firstPath, secondPath}, sessionPath)
	first, second := state.space.Documents[0], state.space.Documents[1]
	first.Editor.MoveTo(0, false)
	first.Editor.Insert("first draft: ")
	first.Editor.SetSelection(text.Range{Start: 0, End: 5})
	second.Editor.MoveTo(0, false)
	second.Editor.Insert("second draft: ")
	second.Editor.SetSelection(text.Range{Start: 1, End: 6})
	state.snapshotRecovery()
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(firstPath, filepath.Join(root, "moved-first.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secondPath, firstPath); err != nil {
		if os.IsPermission(err) {
			t.Skipf("file symlinks are unavailable: %v", err)
		}
		t.Fatal(err)
	}

	restored := newStartupModel([]string{secondPath}, sessionPath)
	if len(restored.space.Documents) != 2 {
		t.Fatalf("recovery collapsed two drafts into %d tab(s)", len(restored.space.Documents))
	}
	attached, detached := restored.space.Documents[0], restored.space.Documents[1]
	if restored.space.Current() != attached || attached.Path != second.Path || attached.Editor.String() != second.Editor.String() ||
		attached.Editor.Selection() != second.Editor.Selection() || attached.EOLSequence() != "WL" || !attached.Dirty() {
		t.Fatalf("original path did not retain its own draft: %q, text=%q", attached.Path, attached.Editor.String())
	}
	if detached.Path != "" || detached.Name != "Recovered first.txt" || detached.Editor.String() != first.Editor.String() ||
		detached.Editor.Selection() != first.Editor.Selection() || detached.EOLSequence() != "WL" || !detached.Dirty() {
		t.Fatalf("redirected path did not recover as a separate draft: %q, text=%q", detached.Path, detached.Editor.String())
	}
	for _, path := range []string{secondPath, filepath.Join(root, "moved-first.txt")} {
		if data, err := os.ReadFile(path); err != nil || string(data) != disk {
			t.Fatalf("recovery changed saved bytes at %q: %q, %v", path, data, err)
		}
	}
}

func TestSinglePaneRecoveryDoesNotOpenRedirectedFile(t *testing.T) {
	for _, name := range []string{"file", "parent directory"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			sourceDir, targetDir := filepath.Join(root, "source"), filepath.Join(root, "target")
			for _, dir := range []string{sourceDir, targetDir} {
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			leftPath := filepath.Join(root, "left.txt")
			sourcePath, targetPath := filepath.Join(sourceDir, "draft.txt"), filepath.Join(targetDir, "draft.txt")
			const saved = "original\r\nlast\n"
			for _, path := range []string{leftPath, sourcePath, targetPath} {
				if err := os.WriteFile(path, []byte(saved), 0600); err != nil {
					t.Fatal(err)
				}
			}
			sessionPath := filepath.Join(root, "settings", "session.json")
			state := newStartupModel([]string{leftPath, sourcePath}, sessionPath)
			state.toggleSplit()
			state.activatePane(1)
			draft := state.space.Current()
			if draft.Name != "draft.txt" || state.paneOwner[draft] != 1 {
				t.Fatal("fixture did not put the saved document in the right pane")
			}
			draft.Editor.MoveTo(0, false)
			draft.Editor.Insert("unsaved: ")
			state.toggleSplit()
			selection := text.Range{Start: 1, End: 5}
			draft.Editor.SetSelection(selection)
			state.snapshotRecovery()
			if err := state.recoveryWriter.Wait(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
				t.Fatalf("fixture unexpectedly has a saved session: %v", err)
			}

			from, to := sourcePath, targetPath
			if name == "parent directory" {
				from, to = sourceDir, targetDir
			}
			parked := filepath.Join(root, "parked")
			if err := os.Rename(from, parked); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(to, from); err != nil {
				if os.IsPermission(err) {
					t.Skipf("symlinks are unavailable: %v", err)
				}
				t.Fatal(err)
			}

			restored := newStartupModel(nil, sessionPath)
			if len(restored.space.Documents) != 1 {
				t.Errorf("crash recovery opened %d tabs; want only the recovered draft", len(restored.space.Documents))
			}
			doc := restored.space.Current()
			if restored.splitEnabled || doc.Path != "" || doc.Name != "Recovered draft.txt" || !doc.Dirty() ||
				doc.Editor.String() != "unsaved: original\nlast\n" || doc.Editor.Selection() != selection || doc.EOLSequence() != "WL" {
				t.Fatalf("recovered path=%q, text=%q, selection=%+v, endings=%q", doc.Path, doc.Editor.String(), doc.Editor.Selection(), doc.EOLSequence())
			}
			parkedFile := parked
			if name == "parent directory" {
				parkedFile = filepath.Join(parked, "draft.txt")
			}
			for _, path := range []string{targetPath, parkedFile} {
				if data, err := os.ReadFile(path); err != nil || string(data) != saved {
					t.Fatalf("recovery changed saved bytes at %q: %q, %v", path, data, err)
				}
			}
		})
	}
}

func TestWindowActivationRetriesFailedUnchangedRecovery(t *testing.T) {
	root := t.TempDir()
	settings := filepath.Join(root, "settings")
	state := newStartupModel(nil, filepath.Join(settings, "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	clickID(t, driver, editorID)
	if err := driver.Type("original draft"); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}

	parked := filepath.Join(root, "last-good-settings")
	if err := os.Rename(settings, parked); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type(" updated"); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err == nil {
		t.Fatal("recovery unexpectedly wrote through a non-directory")
	}
	state.observeRecoveryError()
	if !strings.HasPrefix(state.notice, "Recovery failed: ") {
		t.Fatalf("missing failure notice: %q", state.notice)
	}
	if err := os.Remove(settings); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parked, settings); err != nil {
		t.Fatal(err)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	stored, err := recovery.Load(state.recoveryPath)
	if err != nil || len(stored.Tabs) != 1 || stored.Tabs[0].Content != "original draft" {
		t.Fatalf("failure lost the last backup or an idle wake retried it: %+v, %v", stored, err)
	}

	want := state.currentRecovery()
	if err := driver.ActivateWindow(); err != nil {
		t.Fatal(err)
	}
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatalf("activation did not retry recovery: %v", err)
	}
	state.observeRecoveryError()
	stored, err = recovery.Load(state.recoveryPath)
	if err != nil || !recoveryEqual(stored, want) || !recoveryEqual(state.currentRecovery(), want) || state.notice != "" {
		t.Fatalf("retry changed the live state or failed to persist it: backup=%+v notice=%q err=%v", stored, state.notice, err)
	}
}
