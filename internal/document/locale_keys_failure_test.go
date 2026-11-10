package document

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/text"
)

type refusingLocaleKeys struct {
	calls int
	err   error
}

func (keys *refusingLocaleKeys) key(value string) ([]byte, error) {
	keys.calls++
	if keys.calls == 2 {
		return nil, keys.err
	}
	return []byte(value), nil
}

func TestLaterLocaleKeyFailurePreservesDocumentAndRedo(t *testing.T) {
	for _, rectangle := range []bool{false, true} {
		doc := New("key-refused.txt")
		before := "z 22\na 1"
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		doc.Editor.Insert("x")
		if !doc.Editor.Undo() || !doc.Editor.CanRedo() {
			t.Fatal("missing Redo fixture")
		}
		rect := Rectangle{Anchor: 2, Caret: doc.Editor.RuneCount(), TabWidth: 4}
		doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 1})
		selection, caret, dirty := doc.Editor.Selection(), doc.Editor.Caret(), doc.Dirty()
		cursor := doc.Editor.TrackEdits()
		failure := errors.New("later native key failed")
		keys := &refusingLocaleKeys{err: failure}
		var accepted bool
		var err error
		if rectangle {
			var after Rectangle
			after, accepted, err = doc.sortRectangleLocaleKeys(rect, keys, true)
			if after != rect {
				t.Fatal("failed key changed returned rectangle")
			}
		} else {
			accepted, err = doc.sortLinesLocaleKeys(keys, true)
		}
		edits, _, complete := doc.Editor.EditsSince(cursor, nil)
		if accepted || !errors.Is(err, failure) || keys.calls != 2 || doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Dirty() != dirty || doc.Editor.CanUndo() || !doc.Editor.CanRedo() || len(edits) != 0 || !complete {
			t.Fatalf("later key failure partially changed source: rectangle=%v accepted=%v err=%v calls=%d edits=%+v", rectangle, accepted, err, keys.calls, edits)
		}
	}
}
