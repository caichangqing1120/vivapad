package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestConvertCaseOnlyChangesSelectedUnicodeTextAndUndoes(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("pre Ab猫c post")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 4, End: 8})
	if !doc.ConvertCase(CaseUpper) || doc.Editor.String() != "pre AB猫C post" {
		t.Fatalf("uppercase selection = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "pre Ab猫c post" {
		t.Fatalf("uppercase undo = %q", doc.Editor.String())
	}
	doc.Editor.SetSelection(text.Range{Start: 4, End: 8})
	if !doc.ConvertCase(CaseLower) || doc.Editor.String() != "pre ab猫c post" {
		t.Fatalf("lowercase selection = %q", doc.Editor.String())
	}
}

func TestInvertCasePreservesUnselectedAndUncasedRunes(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("OUT aB猫中")
	doc.Editor.SetSelection(text.Range{Start: 4, End: 7})
	if !doc.ConvertCase(CaseInvert) || doc.Editor.String() != "OUT Ab猫中" {
		t.Fatalf("inverted selection = %q", doc.Editor.String())
	}
}

func TestConvertCaseWithoutSelectionOrChangeIsNoOp(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("猫")
	doc.Editor.ClearHistory()
	wasDirty := doc.Dirty()
	if doc.ConvertCase(CaseUpper) || doc.Editor.String() != "猫" || doc.Dirty() != wasDirty || doc.Editor.CanUndo() {
		t.Fatalf("unselected conversion changed text/history: %q", doc.Editor.String())
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	if doc.ConvertCase(CaseUpper) || doc.ConvertCase(CaseMode(99)) || doc.Editor.CanUndo() {
		t.Fatalf("uncased conversion changed history: %q", doc.Editor.String())
	}
}

func TestConvertCasePreservesWindows1252OnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "western.txt")
	if err := os.WriteFile(path, []byte{'c', 'a', 'f', 0xe9}, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := OpenWithEncoding(path, Windows1252)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 4})
	if !doc.ConvertCase(CaseUpper) || doc.Editor.String() != "CAFÉ" {
		t.Fatalf("legacy uppercase = %q", doc.Editor.String())
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != string([]byte{'C', 'A', 'F', 0xc9}) {
		t.Fatalf("legacy save = %x, %v", data, err)
	}
}
