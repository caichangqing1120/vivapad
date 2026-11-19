package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/recovery"
	"vivapad/internal/session"
)

func TestReadOnlyAutomaticSessionAndReloadPreserveFlag(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	firstPath, secondPath := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sessionPath := filepath.Join(root, "data", "session.json")
	state := newStartupModel([]string{firstPath, secondPath}, sessionPath)
	state.space.Select(0)
	state.command("Read-Only on Current Document")
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	stored, err := session.Load(sessionPath)
	if err != nil || !allowed || !stored.ReadOnly[firstPath] || stored.ReadOnly[secondPath] {
		t.Fatalf("read-only session = %+v, allowed=%v, err=%v", stored, allowed, err)
	}
	restored := newStartupModel(nil, sessionPath)
	for _, doc := range restored.space.Documents {
		if restored.isReadOnly(doc) != (doc.Path == firstPath) {
			t.Fatalf("restored read-only state wrong for %s", doc.Path)
		}
	}
	withArgument := newStartupModel([]string{firstPath}, sessionPath)
	if !withArgument.isReadOnly(withArgument.space.Current()) {
		t.Fatal("explicit file argument lost saved read-only flag")
	}
	restored.space.Select(0)
	previous := restored.space.Current()
	if err := os.WriteFile(firstPath, []byte("external change"), 0600); err != nil {
		t.Fatal(err)
	}
	restored.requestReload()
	if restored.space.Current() == previous || !restored.isReadOnly(restored.space.Current()) {
		t.Fatalf("reload lost read-only flag: current=%p previous=%p", restored.space.Current(), previous)
	}
}

func TestReadOnlyDirtyRecoveryRestoresUntitledFlag(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "data", "session.json")
	state := newStartupModel(nil, sessionPath)
	doc := state.space.Current()
	doc.Editor.Insert("draft")
	state.command("Read-Only on Current Document")
	state.snapshotRecovery()
	if err := state.recoveryWriter.Wait(); err != nil {
		t.Fatal(err)
	}
	stored, err := recovery.Load(state.recoveryPath)
	if err != nil || len(stored.Tabs) != 1 || !stored.Tabs[0].ReadOnly {
		t.Fatalf("recovery backup = %+v, %v", stored, err)
	}
	restored := newStartupModel(nil, sessionPath)
	if !restored.isReadOnly(restored.space.Current()) || restored.space.Current().Editor.String() != "draft" || !restored.space.Current().Dirty() {
		t.Fatalf("read-only recovery = %+v", restored.space.Current())
	}
}

func TestDirtyRecoveryKeepsFilePermissionsSeparateFromManualReadOnly(t *testing.T) {
	for _, tc := range []struct {
		name   string
		manual bool
	}{{"file attribute only", false}, {"with manual protection", true}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path, sessionPath := filepath.Join(root, "file.txt"), filepath.Join(root, "data", "session.json")
			if err := os.WriteFile(path, []byte("saved\r\n"), 0600); err != nil {
				t.Fatal(err)
			}
			state := newStartupModel([]string{path}, sessionPath)
			doc := state.space.Current()
			doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
			doc.Editor.Insert("draft")
			state.readOnly[doc] = tc.manual
			if err := os.Chmod(path, 0400); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0600) })
			if !state.isReadOnly(doc) {
				t.Fatal("fixture did not make the document read-only")
			}
			state.snapshotRecovery()
			if err := state.recoveryWriter.Wait(); err != nil {
				t.Fatal(err)
			}
			stored, err := recovery.Load(state.recoveryPath)
			if err != nil || len(stored.Tabs) != 1 {
				t.Fatalf("recovery backup = %+v, %v", stored, err)
			}
			if stored.Tabs[0].ReadOnly != tc.manual {
				t.Errorf("backup manual flag = %t, want %t", stored.Tabs[0].ReadOnly, tc.manual)
			}
			if err := os.Chmod(path, 0600); err != nil {
				t.Fatal(err)
			}
			restored := newStartupModel(nil, sessionPath)
			recovered := restored.space.Current()
			if recovered.Path != doc.Path || recovered.Editor.String() != "saved\ndraft" || !recovered.Dirty() || !recovered.Editor.CanUndo() {
				t.Fatal("recovery lost the source binding, draft, or Undo history")
			}
			if restored.isReadOnly(recovered) != tc.manual {
				t.Errorf("restored editor read-only = %t, want %t", restored.isReadOnly(recovered), tc.manual)
			}
			driver, err := app.NewDriver(newConfig(restored), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, editorID)
			recovered.Editor.MoveTo(recovered.Editor.RuneCount(), false)
			if tc.manual {
				_ = driver.Type("x")
				if recovered.Editor.String() != "saved\ndraft" {
					t.Fatal("typing bypassed restored manual protection")
				}
			} else {
				if err := driver.Type("x"); err != nil || recovered.Editor.String() != "saved\ndraftx" {
					t.Fatalf("typing in the recovered writable draft = %q, %v", recovered.Editor.String(), err)
				}
				if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || recovered.Editor.String() != "saved\ndraft" {
					t.Fatalf("Undo after first key = %q, %v", recovered.Editor.String(), err)
				}
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "saved\r\n" {
				t.Fatalf("recovery or typing changed disk: %q, %v", data, err)
			}
		})
	}
}
