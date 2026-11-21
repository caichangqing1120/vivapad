//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/text"
	"golang.org/x/sys/unix"
	"vivapad/internal/session"
)

func TestLoadNPPRejectsReferencedPipeWithoutChangingWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name     string
		codePage int
	}{
		{"automatic encoding", -1},
		{"explicit legacy encoding", 54936},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			valid, pipe := filepath.Join(root, "valid.txt"), filepath.Join(root, "pipe.txt")
			if err := os.WriteFile(valid, []byte("valid\r\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := unix.Mkfifo(pipe, 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "session.xml")
			if err := session.SaveNPP(path, session.NPPState{
				Main: []session.NPPFile{{Path: valid, Encoding: -1}},
				Sub:  []session.NPPFile{{Path: pipe, Encoding: tc.codePage}}, ActiveView: 1,
			}); err != nil {
				t.Fatal(err)
			}
			state, driver := startDriver(t)
			draft := state.space.Current()
			draft.Editor.Insert("keep draft")
			selection := text.Range{Start: 2, End: 6}
			draft.Editor.SetSelection(selection)
			if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(0))
			clickID(t, driver, menuItemScope.At(13))
			done := make(chan struct{})
			go func() {
				defer close(done)
				clickID(t, driver, sessionScope.At(0))
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				// Release a regressed reader so Driver.Close and cleanup can run.
				if fd, err := unix.Open(pipe, unix.O_WRONLY|unix.O_NONBLOCK, 0); err == nil {
					unix.Close(fd)
					<-done
				}
				t.Fatal("session import waited for a writer on a referenced pipe")
			}
			if len(state.space.Documents) != 1 || state.space.Current() != draft || state.splitEnabled ||
				!draft.Dirty() || !draft.Editor.CanUndo() || draft.Editor.String() != "keep draft" || draft.Editor.Selection() != selection ||
				len(state.recentEntries) != 0 || !strings.Contains(state.notice, "pipe.txt") || !strings.Contains(state.notice, "not a regular file") {
				t.Fatalf("rejected import changed the workspace: tabs=%d split=%t notice=%q", len(state.space.Documents), state.splitEnabled, state.notice)
			}
		})
	}
}
