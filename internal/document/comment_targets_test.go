package document

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"unicode"

	"github.com/vivaui/viva/text"
)

func TestCommentTargetsOnlyChangeDelimitersAndRestoreRedoSelection(t *testing.T) {
	for _, scenario := range []struct {
		name, before, after string
		apply               func(*Document) bool
		want                []text.Edit
	}{
		{"line add", "  α\n\tβ\n", "  // α\n\t// β\n", func(doc *Document) bool { return doc.ChangeLineComments("//", LineCommentAdd) },
			[]text.Edit{{Start: 2, Inserted: 3}, {Start: 9, Inserted: 3}}},
		{"stream line add", "α\nβ", "/* α */\n/* β */", func(doc *Document) bool { return doc.ChangeStreamLineComments("/*", "*/", LineCommentAdd) },
			[]text.Edit{{Start: 0, Inserted: 3}, {Start: 5, Inserted: 3}, {Start: 9, Inserted: 3}, {Start: 14, Inserted: 3}}},
		{"line remove", "// α\n// β", "α\nβ", func(doc *Document) bool { return doc.ChangeLineComments("//", LineCommentRemove) },
			[]text.Edit{{Start: 0, Removed: 3}, {Start: 3, Removed: 3}}},
		{"block add", "α\nβ", "/* α\nβ */", func(doc *Document) bool { return doc.AddBlockComment("/*", "*/") },
			[]text.Edit{{Start: 0, Inserted: 3}, {Start: 8, Inserted: 3}}},
		{"block remove", "/* α */ gap /* β */", "α gap β", func(doc *Document) bool { return doc.RemoveBlockComments("/*", "*/") },
			[]text.Edit{{Start: 0, Removed: 3}, {Start: 2, Removed: 3}, {Start: 7, Removed: 3}, {Start: 9, Removed: 3}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			doc := New("comments.go")
			doc.Editor.Insert(scenario.before)
			doc.Editor.ClearHistory()
			doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 0})
			beforeSelection := doc.Editor.Selection()
			cursor := doc.Editor.TrackEdits()
			if !scenario.apply(doc) || doc.Editor.String() != scenario.after {
				t.Fatalf("comment result = %q, want %q", doc.Editor.String(), scenario.after)
			}
			edits, _, complete := doc.Editor.EditsSince(cursor, nil)
			if !complete || !slices.Equal(edits, scenario.want) {
				t.Fatalf("comment primitives = %+v complete=%v, want %+v", edits, complete, scenario.want)
			}
			afterSelection, afterCaret := doc.Editor.Selection(), doc.Editor.Caret()
			if !doc.Editor.Undo() || doc.Editor.String() != scenario.before || doc.Editor.Selection() != beforeSelection || doc.Editor.Caret() != 0 || doc.Editor.CanUndo() {
				t.Fatal("comment Undo did not restore the original directional selection in one step")
			}
			if !doc.Editor.Redo() || doc.Editor.String() != scenario.after || doc.Editor.Selection() != afterSelection || doc.Editor.Caret() != afterCaret {
				t.Fatalf("comment Redo selection = %+v caret=%d, want %+v caret=%d", doc.Editor.Selection(), doc.Editor.Caret(), afterSelection, afterCaret)
			}
		})
	}
}

func TestCommentTargetsRefuseCompositionAndRestrictionsAtomically(t *testing.T) {
	for _, scenario := range []struct {
		name string
		set  func(*text.Editor)
	}{
		{"composition", func(editor *text.Editor) { editor.SetMarkedText("ni", text.Range{Start: 2, End: 2}) }},
		{"restricted", func(editor *text.Editor) { editor.Restrict(unicode.IsLetter, 100) }},
	} {
		for _, command := range []struct {
			name  string
			apply func(*Document) bool
		}{
			{"line", func(doc *Document) bool { return doc.ChangeLineComments("//", LineCommentAdd) }},
			{"stream line", func(doc *Document) bool { return doc.ChangeStreamLineComments("/*", "*/", LineCommentToggle) }},
			{"block", func(doc *Document) bool { return doc.AddBlockComment("/*", "*/") }},
			{"unblock", func(doc *Document) bool { return doc.RemoveBlockComments("/*", "*/") }},
		} {
			t.Run(scenario.name+"/"+command.name, func(t *testing.T) {
				doc := New("refused.go")
				doc.Editor.Insert("/* alpha */\nbeta")
				doc.Editor.ClearHistory()
				doc.Editor.SetSelection(text.Range{Start: 3, End: 8})
				scenario.set(doc.Editor)
				before, selection, caret := doc.Editor.String(), doc.Editor.Selection(), doc.Editor.Caret()
				canUndo, canRedo := doc.Editor.CanUndo(), doc.Editor.CanRedo()
				composition, active := doc.Editor.Composition()
				cursor := doc.Editor.TrackEdits()
				if command.apply(doc) || doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Editor.CanUndo() != canUndo || doc.Editor.CanRedo() != canRedo {
					t.Fatal("refused comment changed text, selection or history")
				}
				if got, stillActive := doc.Editor.Composition(); got != composition || active != stillActive {
					t.Fatal("refused comment changed composition")
				}
				if edits, _, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
					t.Fatal("refused comment changed the edit journal")
				}
			})
		}
	}
}

func TestCommentTargetsPreserveEmptyBodyAndMixedEOL(t *testing.T) {
	for _, value := range []string{"", "  ", "\n"} {
		doc := New("empty.go")
		doc.Editor.Insert(value)
		doc.Editor.ClearHistory()
		doc.Editor.MoveTo(0, false)
		if !doc.AddBlockComment("/*", "*/") || !doc.Editor.Undo() || doc.Editor.String() != value {
			t.Fatalf("empty-body block comment failed for %q", value)
		}
	}
	path := filepath.Join(t.TempDir(), "mixed.go")
	before := "α\r\nβ\rγ\n"
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	if !doc.ChangeLineComments("//", LineCommentAdd) || doc.EOLSequence() != "WML" {
		t.Fatal("line comment changed mixed separators")
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(path); err != nil || string(saved) != "// α\r\n// β\r// γ\n" {
		t.Fatalf("saved comment bytes = %q, %v", saved, err)
	}
	if !doc.Editor.Undo() || doc.EOLSequence() != "WML" || doc.Editor.String() != "α\nβ\nγ\n" {
		t.Fatal("line comment Undo lost mixed separators")
	}
}
