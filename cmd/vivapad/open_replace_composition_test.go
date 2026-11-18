package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestOpenReplacementPreservesLiveDocumentComposition(t *testing.T) {
	for _, entry := range []struct {
		name string
		run  func(*model)
	}{
		{"menu command", func(state *model) { state.command("Replace All in Open Documents") }},
		{"find strip action", (*model).replaceAllOpenDocuments},
	} {
		for _, source := range []string{"cat\nanchor", "dog\nanchor"} {
			busy := strings.HasPrefix(source, "cat")
			name := "skipped composing tab"
			if busy {
				name = "affected composing tab"
			}
			t.Run(entry.name+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				peerPath, activePath := filepath.Join(root, "peer.txt"), filepath.Join(root, "active.txt")
				for path, value := range map[string]string{peerPath: "cat peer", activePath: source} {
					if err := os.WriteFile(path, []byte(value), 0600); err != nil {
						t.Fatal(err)
					}
				}
				state := newModelWithFiles([]string{peerPath, activePath})
				peer, active := state.space.Documents[0], state.space.Documents[1]
				state.splitEnabled = true
				state.paneDocs = [2]*document.Document{active, active}
				state.activatePane(1)
				state.setBookmarkLines(active, []int{1})
				state.query.Insert("cat")
				state.replacement.Insert("fox")
				active.Editor.MoveTo(active.Editor.RuneCount(), false)
				input := liveDocumentCompositionInput(t, state, 1)
				sendComposition(t, input, text.CompositionUpdate, "pending")
				assertLiveComposition(t, active)
				before, cursor := captureComposition(state, active), active.Editor.TrackLineEdits()

				entry.run(state)

				if after := captureComposition(state, active); !reflect.DeepEqual(after, before) || active.Editor.TrackLineEdits() != cursor {
					t.Fatalf("replacement changed the live document: before=%+v after=%+v", before, after)
				}
				if busy {
					if !strings.Contains(state.notice, "composition") || peer.Editor.String() != "cat peer" || peer.Editor.CanUndo() {
						t.Fatalf("batch did not refuse atomically: peer=%q notice=%q", peer.Editor.String(), state.notice)
					}
				} else if peer.Editor.String() != "fox peer" || !strings.Contains(state.notice, "1 open tabs") {
					t.Fatalf("unaffected composing tab blocked its peer: peer=%q notice=%q", peer.Editor.String(), state.notice)
				}
				sendComposition(t, input, text.CompositionCancel, "")
				if active.Editor.String() != source || active.Dirty() {
					t.Fatalf("composition can no longer roll back: %q", active.Editor.String())
				}
				if busy {
					entry.run(state)
					if active.Editor.String() != "fox\nanchor" || peer.Editor.String() != "fox peer" {
						t.Fatal("replacement could not be retried after composition ended")
					}
				}
				for path, want := range map[string]string{peerPath: "cat peer", activePath: source} {
					if bytes, err := os.ReadFile(path); err != nil || string(bytes) != want {
						t.Fatalf("batch wrote %s: %q, %v", filepath.Base(path), bytes, err)
					}
				}
			})
		}
	}
}
