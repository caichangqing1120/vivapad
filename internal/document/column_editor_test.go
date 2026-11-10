package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestColumnEditorTextUsesVisualColumnsAndOneUndo(t *testing.T) {
	doc := New("new 1")
	before := "a\tb\n界c\nz\n"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 2, End: 2})
	changed, err := doc.EditColumn(ColumnEdit{Text: "X"}, 4)
	if err != nil || !changed || doc.Editor.String() != "a\tXb\n界c X\nz   X\n    X" {
		t.Fatalf("column text = %q, changed=%v, err=%v", doc.Editor.String(), changed, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("column text undo = %q", doc.Editor.String())
	}
}

func TestColumnEditorNumbersRepeatFormatsAndValidateAtomically(t *testing.T) {
	for _, scenario := range []struct {
		base    int
		initial string
		repeat  string
		upper   bool
		leading ColumnLeading
		want    string
	}{
		{10, "9", "2", false, ColumnLeadingZero, "a09\nb09\nc10\nd10\ne11"},
		{16, "f", "2", true, ColumnLeadingSpace, "a F\nb F\nc10\nd10\ne11"},
		{8, "7", "2", false, ColumnLeadingNone, "a7\nb7\nc10\nd10\ne11"},
		{2, "1", "10", false, ColumnLeadingNone, "a1\nb1\nc10\nd10\ne11"},
	} {
		doc := New("new 1")
		doc.Editor.Insert("a\nb\nc\nd\ne")
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(text.Range{Start: 1, End: 1})
		changed, err := doc.EditColumn(ColumnEdit{Number: true, Initial: scenario.initial, Increment: "1", Repeat: scenario.repeat, Base: scenario.base, Uppercase: scenario.upper, Leading: scenario.leading}, 4)
		if err != nil || !changed || doc.Editor.String() != scenario.want {
			t.Fatalf("base %d number edit = %q, changed=%v, err=%v", scenario.base, doc.Editor.String(), changed, err)
		}
		if !doc.Editor.Undo() || doc.Editor.String() != "a\nb\nc\nd\ne" {
			t.Fatalf("base %d did not undo", scenario.base)
		}
	}
	doc := New("new 1")
	doc.Editor.Insert("a\nb\nc")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: 0})
	for _, input := range []ColumnEdit{
		{Text: ""},
		{Text: "a\nb"},
		{Number: true, Initial: "-1", Increment: "1", Repeat: "1", Base: 10},
		{Number: true, Initial: "0", Increment: "1", Repeat: "0", Base: 10},
		{Number: true, Initial: "2147483647", Increment: "1", Repeat: "1", Base: 10},
	} {
		if changed, err := doc.EditColumn(input, 4); err == nil || changed || doc.Editor.String() != "a\nb\nc" || doc.Editor.CanUndo() {
			t.Fatalf("invalid column edit %+v changed=%v err=%v text=%q", input, changed, err, doc.Editor.String())
		}
	}
}

func TestColumnEditorPreservesMixedLineEndings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("a\tb\r\n界c\nz\r"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 2, End: 2})
	if changed, err := doc.EditColumn(ColumnEdit{Text: "X"}, 4); err != nil || !changed {
		t.Fatalf("mixed column edit changed=%v err=%v", changed, err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\tXb\r\n界c X\nz   X\r    X" {
		t.Fatalf("mixed column save = %q, %v", data, err)
	}
}
