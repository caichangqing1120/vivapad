package macro

import (
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestPlaybackCommitsActualTargetsAndRedoSelection(t *testing.T) {
	doc := document.New("macro.txt")
	doc.Editor.Insert("abc")
	doc.Editor.ClearHistory()
	cursor := doc.Editor.TrackEdits()
	steps := []Step{
		{Kind: TextStep, TextKind: app.TextPaste, Text: "XY"},
		{Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextDeleteBackward},
		{Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextMoveLeft},
		{Kind: TextStep, TextKind: app.TextInsert, Text: "Z"},
	}
	if err := Play(doc, steps, 1); err != nil || doc.Editor.String() != "abcZX" || doc.Editor.Caret() != 4 {
		t.Fatalf("macro target result = %q caret=%d: %v", doc.Editor.String(), doc.Editor.Caret(), err)
	}
	edits, _, complete := doc.Editor.EditsSince(cursor, nil)
	want := []text.Edit{{Start: 3, Inserted: 2}, {Start: 4, Removed: 1}, {Start: 3, Inserted: 1}}
	if !complete || !slices.Equal(edits, want) {
		t.Fatalf("macro changed untouched bytes: %+v complete=%v, want %+v", edits, complete, want)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "abc" || doc.Editor.Caret() != 3 || doc.Editor.CanUndo() {
		t.Fatal("macro Undo did not restore one original selection unit")
	}
	if !doc.Editor.Redo() || doc.Editor.String() != "abcZX" || doc.Editor.Caret() != 4 {
		t.Fatal("macro Redo did not restore its final caret")
	}
}

func TestPlaybackPrimitiveLimitDoesNotCommitARepeatedSmallBuffer(t *testing.T) {
	doc := document.New("primitive-limit.txt")
	doc.Editor.Insert("keep")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	cursor := doc.Editor.TrackEdits()
	err := PlayWithExecutor(doc, []Step{{Kind: TextStep, TextKind: app.TextInsert, Text: "!"}}, 1, func(preview *document.Document, _ Step) {
		for index := 0; index <= maxPlaybackSteps; index++ {
			value := "a"
			if index%2 != 0 {
				value = "b"
			}
			preview.Editor.ReplaceRange(text.Range{End: 1}, value)
			preview.Editor.ClearHistory()
		}
	})
	if err == nil || !strings.Contains(err.Error(), "primitive") || doc.Editor.String() != "keep" || doc.Editor.Caret() != 0 || doc.Editor.CanUndo() {
		t.Fatalf("primitive-limit replay changed its source: %q caret=%d, %v", doc.Editor.String(), doc.Editor.Caret(), err)
	}
	if edits, _, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
		t.Fatal("primitive-limit refusal changed the source journal")
	}
}

func BenchmarkPlaybackTargets(b *testing.B) {
	value := strings.Repeat("alpha keep beta\n", 256)
	steps := []Step{
		{Kind: TextStep, TextKind: app.TextInsert, Text: "!"},
		{Kind: TextStep, TextKind: app.TextCommandEvent, Command: app.TextMoveRight},
	}
	b.ReportAllocs()
	for range b.N {
		doc := document.New("benchmark.txt")
		doc.Editor.Insert(value)
		doc.Editor.ClearHistory()
		doc.Editor.MoveTo(0, false)
		if err := Play(doc, steps, 16); err != nil {
			b.Fatal(err)
		}
	}
}

func TestPlaybackEqualTextStillAppliesTargetIndicatorMutations(t *testing.T) {
	doc := document.New("equal.txt")
	doc.Editor.Insert("aaaa")
	doc.Editor.ClearHistory()
	doc.Editor.Insert("!")
	doc.Editor.Undo()
	doc.Editor.SetSelection(text.Range{Start: 3, End: 4})
	cursor := doc.Editor.TrackEdits()
	if err := Play(doc, []Step{{Kind: TextStep, TextKind: app.TextPaste, Text: "a"}}, 1); err != nil {
		t.Fatal(err)
	}
	edits, _, complete := doc.Editor.EditsSince(cursor, nil)
	if !complete || !slices.Equal(edits, []text.Edit{{Start: 3, Removed: 1, Inserted: 1}}) || doc.Editor.String() != "aaaa" || !doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatal("equal-text macro failed to retain its edit or discard its former Redo branch")
	}
	if !doc.Editor.Undo() || doc.Editor.CanUndo() || !doc.Editor.CanRedo() {
		t.Fatal("equal-text macro Undo did not return to its saved boundary")
	}
}

func TestPlaybackRestrictedSourceIsRefusedBeforeItsExecutor(t *testing.T) {
	doc := document.New("restricted.txt")
	doc.Editor.Insert("keep")
	doc.Editor.ClearHistory()
	doc.Editor.Restrict(unicode.IsLetter, 20)
	cursor := doc.Editor.TrackEdits()
	called := false
	err := PlayWithExecutor(doc, []Step{{Kind: TextStep, TextKind: app.TextInsert, Text: "!"}}, 1, func(preview *document.Document, step Step) {
		called = true
		preview.Editor.Insert(step.Text)
	})
	if err == nil || called || doc.Editor.String() != "keep" || doc.Editor.Caret() != 4 || doc.Editor.CanUndo() {
		t.Fatal("restricted macro ran its executor or changed its source")
	}
	if edits, _, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
		t.Fatal("restricted macro changed the source journal")
	}
}
