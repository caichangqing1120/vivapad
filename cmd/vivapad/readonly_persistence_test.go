package main

import (
	"os"
	"path/filepath"
	"testing"

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
