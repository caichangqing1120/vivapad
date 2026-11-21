package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
)

func TestSaveChoosersPreserveOpenFileAliases(t *testing.T) {
	for _, item := range []struct {
		name, wantNotice string
		copy, own, stale bool
	}{
		{name: "save as other tab", wantNotice: "already open in another tab"},
		{name: "copy over other tab", copy: true, wantNotice: "already open in a tab"},
		{name: "copy over source", copy: true, own: true, wantNotice: "already open in a tab"},
		{name: "save as source changed on disk", own: true, stale: true, wantNotice: "changed on disk"},
	} {
		t.Run(item.name, func(t *testing.T) {
			root := t.TempDir()
			source, other, alias := filepath.Join(root, "source.txt"), filepath.Join(root, "other.txt"), filepath.Join(root, "alias.txt")
			const saved = "original\r\n"
			for _, path := range []string{source, other} {
				if err := os.WriteFile(path, []byte(saved), 0600); err != nil {
					t.Fatal(err)
				}
			}
			protected := other
			if item.own {
				protected = source
			}
			if err := os.Link(protected, alias); err != nil {
				t.Skipf("hard links unavailable: %v", err)
			}
			state := newModelWithFiles([]string{source, other})
			state.space.Select(0)
			doc := state.space.Current()
			originalPath, originalDigest := doc.Path, doc.DiskDigest()
			doc.Editor.Insert("draft ")
			beforeText, beforeSelection := doc.Editor.String(), doc.Editor.Selection()
			protectedBytes := saved
			if item.stale {
				protectedBytes = "external\r\n"
				if err := os.WriteFile(protected, []byte(protectedBytes), 0600); err != nil {
					t.Fatal(err)
				}
			}
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			if err := driver.QueueSaveFile(alias, nil); err != nil {
				t.Fatal(err)
			}
			var accepted bool
			if item.copy {
				accepted = state.saveCopy(0)
			} else {
				accepted = state.save(0, true)
			}
			if accepted || !strings.Contains(state.notice, item.wantNotice) {
				t.Errorf("aliased destination was not refused: accepted=%v notice=%q", accepted, state.notice)
			}
			for _, path := range []string{protected, alias} {
				if got, err := os.ReadFile(path); err != nil || string(got) != protectedBytes {
					t.Errorf("protected bytes changed at %s: %q, %v", path, got, err)
				}
			}
			if state.space.Current() != doc || doc.Path != originalPath || doc.DiskDigest() != originalDigest ||
				!doc.Dirty() || !doc.Editor.CanUndo() || doc.Editor.String() != beforeText || doc.Editor.Selection() != beforeSelection {
				t.Fatal("failed save changed the active draft or its history")
			}
		})
	}
}
