package searchfiles

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestOpenReplaceUsesSourceTargetsAndCurrentApplySelection(t *testing.T) {
	first, second := document.New("first"), document.New("second")
	first.Editor.Insert("cat keep cat")
	second.Editor.Insert("cat stay cat")
	first.Editor.ClearHistory()
	second.Editor.ClearHistory()
	firstCursor, secondCursor := first.Editor.TrackEdits(), second.Editor.TrackEdits()
	plan, err := PreviewOpenReplace([]*document.Document{first, second}, "cat", "X", document.SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	first.Editor.MoveTo(5, false)
	if err := plan.Apply([]*document.Document{first, second}); err != nil {
		t.Fatal(err)
	}
	want := []text.Edit{{Start: 0, Removed: 3, Inserted: 1}, {Start: 7, Removed: 3, Inserted: 1}}
	for _, source := range []struct {
		doc    *document.Document
		cursor uint64
	}{{first, firstCursor}, {second, secondCursor}} {
		edits, _, complete := source.doc.Editor.EditsSince(source.cursor, nil)
		if !complete || !slices.Equal(edits, want) {
			t.Fatalf("%s was not a source target batch: %+v", source.doc.Name, edits)
		}
	}
	if first.Editor.String() != "X keep X" || second.Editor.String() != "X stay X" || first.Editor.Caret() != 3 {
		t.Fatalf("current apply selection not mapped: %q / %q caret %d", first.Editor.String(), second.Editor.String(), first.Editor.Caret())
	}
	if !first.Editor.Undo() || first.Editor.Caret() != 5 || first.Editor.String() != "cat keep cat" || first.Editor.CanUndo() || second.Editor.String() != "X stay X" {
		t.Fatal("target apply did not retain independent current-selection Undo")
	}
	if err := plan.Apply([]*document.Document{first, second}); err == nil {
		t.Fatal("open target plan replayed after successful apply")
	}
}

func TestOpenReplacePreflightsLaterEditorConstraintsBeforeAnyMutation(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		set   func(*text.Editor)
		clear func(*text.Editor)
	}{
		{"composition", func(editor *text.Editor) { editor.SetMarkedText("", text.Range{}) }, func(editor *text.Editor) { editor.Unmark() }},
		{"restriction", func(editor *text.Editor) { editor.Restrict(func(rune) bool { return true }, 0) }, func(editor *text.Editor) { editor.Restrict(nil, 0) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			first, second := document.New("first"), document.New("second")
			first.Editor.Insert("cat")
			first.Editor.ClearHistory()
			first.Editor.Insert("!")
			first.Editor.Undo()
			second.Editor.Insert("cat")
			second.Editor.ClearHistory()
			second.Editor.MoveTo(1, false)
			scenario.set(second.Editor)
			plan, err := PreviewOpenReplace([]*document.Document{first, second}, "cat", "dog", document.SearchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			firstCursor, secondCursor := first.Editor.TrackEdits(), second.Editor.TrackEdits()
			firstSelection, secondSelection := first.Editor.Selection(), second.Editor.Selection()
			if err := plan.Apply([]*document.Document{first, second}); err == nil {
				t.Fatal("later editor constraints were ignored")
			}
			if first.Editor.String() != "cat" || second.Editor.String() != "cat" || first.Editor.Selection() != firstSelection || second.Editor.Selection() != secondSelection || !first.Editor.CanRedo() || first.Editor.CanUndo() || second.Editor.CanUndo() {
				t.Fatal("later preflight refusal partially mutated an earlier tab/history")
			}
			for _, source := range []struct {
				editor *text.Editor
				cursor uint64
			}{{first.Editor, firstCursor}, {second.Editor, secondCursor}} {
				if edits, next, complete := source.editor.EditsSince(source.cursor, nil); !complete || len(edits) != 0 || next != source.cursor {
					t.Fatal("refused open plan entered an edit journal")
				}
			}
			scenario.clear(second.Editor)
			if err := plan.Apply([]*document.Document{first, second}); err != nil || first.Editor.String() != "dog" || second.Editor.String() != "dog" {
				t.Fatalf("refused plan was not retryable: %q / %q, %v", first.Editor.String(), second.Editor.String(), err)
			}
		})
	}
}

func TestOpenReplaceDeduplicatesClonesAndRefusesAmbiguousEditors(t *testing.T) {
	doc := document.New("shared")
	doc.Editor.Insert("cat cat")
	doc.Editor.ClearHistory()
	plan, err := PreviewOpenReplace([]*document.Document{doc, doc}, "cat", "dog", document.SearchOptions{})
	if err != nil || len(plan.Changes) != 1 || plan.Replacements != 2 {
		t.Fatalf("shared clone counted twice: %+v, %v", plan, err)
	}
	if err := plan.Apply([]*document.Document{doc, doc}); err != nil || doc.Editor.String() != "dog dog" || !doc.Editor.Undo() || doc.Editor.CanUndo() {
		t.Fatalf("shared source did not apply once: %q, %v", doc.Editor.String(), err)
	}
	other := document.New("ambiguous")
	other.Editor = doc.Editor
	if _, err := PreviewOpenReplace([]*document.Document{doc, other}, "cat", "dog", document.SearchOptions{}); err == nil {
		t.Fatal("distinct documents sharing an editor were accepted")
	}
}

func TestOpenReplaceRefusesTamperedTargetMetadataBeforeAnyMutation(t *testing.T) {
	for _, kind := range []string{"edits", "source", "shared editor", "count", "total", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			first, second, third := document.New("first"), document.New("second"), document.New("third")
			for _, doc := range []*document.Document{first, second, third} {
				doc.Editor.Insert("cat")
				doc.Editor.ClearHistory()
			}
			plan, err := PreviewOpenReplace([]*document.Document{first, second}, "cat", "dog", document.SearchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "edits":
				plan.Changes[1].Edits[0].Text = "wrong"
			case "source":
				plan.Changes[1].Doc = third
			case "shared editor":
				second.Editor = first.Editor
			case "count":
				plan.Changes[1].Count++
			case "total":
				plan.Replacements++
			case "duplicate":
				plan.Changes = append(plan.Changes, plan.Changes[0])
			}
			if err := plan.Apply([]*document.Document{first, second, third}); err == nil {
				t.Fatal("tampered or ambiguous source plan applied")
			}
			for _, doc := range []*document.Document{first, second, third} {
				if doc.Editor.String() != "cat" || doc.Editor.CanUndo() {
					t.Fatalf("tampered plan partially changed %s", doc.Name)
				}
			}
		})
	}
}

func TestOpenReplaceRefusesUnavailableEditorWithoutPanic(t *testing.T) {
	first, second := document.New("first"), document.New("second")
	first.Editor.Insert("cat")
	second.Editor.Insert("cat")
	first.Editor.ClearHistory()
	second.Editor.ClearHistory()
	plan, err := PreviewOpenReplace([]*document.Document{first, second}, "cat", "dog", document.SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second.Editor = nil
	if err := plan.Apply([]*document.Document{first, second}); err == nil || first.Editor.String() != "cat" || first.Editor.CanUndo() {
		t.Fatalf("missing later editor changed earlier source: %q, %v", first.Editor.String(), err)
	}
	if _, err := PreviewOpenReplace([]*document.Document{second}, "cat", "dog", document.SearchOptions{}); err == nil {
		t.Fatal("missing source editor was previewed")
	}
}
