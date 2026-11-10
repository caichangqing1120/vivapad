package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestReloadCapturesLineHistoryBeforeFirstBookmark(t *testing.T) {
	for _, discard := range []bool{false, true} {
		name := "reload"
		if discard {
			name = "discard legacy"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "reload.txt")
			if err := os.WriteFile(path, []byte("aa\nbb"), 0600); err != nil {
				t.Fatal(err)
			}
			space := New()
			var err error
			if discard {
				err = space.OpenWithEncoding(path, document.Windows1252)
			} else {
				err = space.Open(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			previous := space.Current()
			if discard {
				previous.Editor.Insert("unsaved")
				_, err = space.ReloadDiscard(0)
			} else {
				_, err = space.Reload(0)
			}
			if err != nil || space.Current() == previous {
				t.Fatalf("reload did not replace document: %v", err)
			}
			editor := space.Current().Editor
			if err := editor.ReplaceRangesWith([]text.Replacement{{Range: text.Range{Start: 1, End: 1}, Text: "\n"}}, text.ReplaceRangesOptions{}); err != nil {
				t.Fatal(err)
			}
			cursor := editor.TrackLineEdits()
			if !editor.Undo() {
				t.Fatal("reloaded document has no batch Undo")
			}
			edits, next, complete := editor.LineEditsSince(cursor, nil)
			if !complete || next != cursor+1 || len(edits) != 1 || edits[0].Line != 0 || edits[0].RemovedLines != 1 || edits[0].InsertedLines != 0 || editor.String() != "aa\nbb" {
				t.Fatalf("reloaded history line facts = %v, cursor %d, complete %v", edits, next, complete)
			}
		})
	}
}
