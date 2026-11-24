package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
)

func TestMonitoringFollowsOnlyItsOriginalPath(t *testing.T) {
	for _, trial := range []struct {
		name     string
		command  string
		rebind   bool
		manual   bool
		choose   bool
		cancel   bool
		badPath  bool
		samePath bool
	}{
		{name: "save as", command: "Save As...", choose: true, rebind: true, manual: true},
		{name: "rename", command: "Rename...", choose: true, rebind: true},
		{name: "save as cancel", command: "Save As...", choose: true, cancel: true},
		{name: "save as failure", command: "Save As...", choose: true, badPath: true},
		{name: "rename cancel", command: "Rename...", choose: true, cancel: true},
		{name: "rename failure", command: "Rename...", choose: true, badPath: true},
		{name: "save as same path", command: "Save As...", choose: true, samePath: true},
		{name: "rename same path", command: "Rename...", choose: true, samePath: true},
		{name: "save", command: "Save"},
		{name: "save a copy", command: "Save a Copy As...", choose: true},
	} {
		t.Run(trial.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(root, "original.log")
			destination := filepath.Join(root, "destination.log")
			const original = "original\r\n"
			if err := os.WriteFile(source, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			if err := state.space.Open(source); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			state.showInActivePane(doc)
			state.command("Clone to Other View")
			if !state.splitEnabled || state.paneDocs[0] != doc || state.paneDocs[1] != doc {
				t.Fatal("test needs a shared split clone")
			}
			pane := state.activePane
			state.readOnly[doc] = trial.manual
			state.toggleMonitoring()
			t.Cleanup(state.stopAllMonitors)
			watcher := state.monitors[doc]
			if watcher == nil {
				t.Fatalf("monitoring did not start: %s", state.notice)
			}
			if trial.choose {
				var response error
				chosen := destination
				if trial.samePath {
					chosen = source
				}
				if trial.badPath {
					chosen = filepath.Join(root, "missing", "destination.log")
				}
				if trial.cancel {
					response = app.ErrCanceled
				}
				if err := driver.QueueSaveFile(chosen, response); err != nil {
					t.Fatal(err)
				}
			}
			state.command(trial.command)
			if state.activePane != pane || state.paneDocs[0] != doc || state.paneDocs[1] != doc || state.readOnly[doc] != trial.manual {
				t.Fatal("operation changed split clone ownership or the manual read-only flag")
			}
			if trial.rebind || trial.command == "Save a Copy As..." {
				if data, err := os.ReadFile(destination); err != nil || string(data) != original {
					t.Fatalf("destination bytes = %q, %v", data, err)
				}
			}
			if trial.rebind && trial.command == "Rename..." {
				if _, err := os.Stat(source); !os.IsNotExist(err) {
					t.Fatalf("rename left the original file: %v", err)
				}
			} else if data, err := os.ReadFile(source); err != nil || string(data) != original {
				t.Fatalf("source bytes = %q, %v", data, err)
			}
			if !trial.rebind {
				if doc.Path != source || state.monitors[doc] != watcher || !state.isReadOnly(doc) {
					t.Fatalf("operation lost the original monitor: path=%q notice=%q", doc.Path, state.notice)
				}
				return
			}
			if doc.Path != destination || state.monitors[doc] != nil || state.isReadOnly(doc) != trial.manual {
				t.Fatalf("path change kept the old monitor or its read-only state: path=%q notice=%q", doc.Path, state.notice)
			}
			select {
			case <-watcher.done:
			default:
				t.Fatal("old monitor still running after rebinding")
			}
			if !strings.Contains(state.notice, "monitoring stopped") {
				t.Fatalf("path change did not report that monitoring stopped: %q", state.notice)
			}
			if err := os.WriteFile(source, []byte("unrelated old-path update"), 0600); err != nil {
				t.Fatal(err)
			}
			watcher.update.Store(monitorFileChanged)
			state.consumeMonitorChanges()
			if state.space.Current() != doc || doc.Editor.String() != "original\n" {
				t.Fatal("an old-path change reloaded the rebound document")
			}
		})
	}
}
