package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestRedactRangesMasksUnicodeCharactersAndPreservesNewlinesAsOneUndo(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("a🙂\n猫b")
	doc.Editor.ClearHistory()
	changed, err := doc.RedactRanges([]text.Range{{Start: 0, End: 5}}, false)
	if err != nil || !changed || doc.Editor.String() != "██\n██" {
		t.Fatalf("Unicode redaction = %q, %v, changed=%v", doc.Editor.String(), err, changed)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a🙂\n猫b" || doc.Editor.CanUndo() {
		t.Fatalf("redaction did not undo atomically: %q", doc.Editor.String())
	}
	if !doc.Editor.Redo() || doc.Editor.String() != "██\n██" {
		t.Fatalf("redaction did not redo: %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() {
		t.Fatal("redaction did not undo for bullet variant")
	}
	changed, err = doc.RedactRanges([]text.Range{{Start: 0, End: 5}}, true)
	if err != nil || !changed || doc.Editor.String() != "●●\n●●" {
		t.Fatalf("Unicode Shift redaction = %q, %v", doc.Editor.String(), err)
	}
}

func TestRedactRangesShiftAndLegacyCodePageUseRepresentableMasks(t *testing.T) {
	doc := New("new 2")
	doc.Editor.Insert("aب\nc")
	doc.SetEncoding(DOS720)
	doc.Editor.ClearHistory()
	changed, err := doc.RedactRanges([]text.Range{{Start: 0, End: 2}, {Start: 3, End: 4}}, false)
	if err != nil || !changed || doc.Editor.String() != "##\n#" {
		t.Fatalf("legacy redaction = %q, %v", doc.Editor.String(), err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "aب\nc" {
		t.Fatalf("legacy redaction undo = %q", doc.Editor.String())
	}
	changed, err = doc.RedactRanges([]text.Range{{Start: 0, End: 2}, {Start: 3, End: 4}}, true)
	if err != nil || !changed || doc.Editor.String() != "..\n." {
		t.Fatalf("legacy Shift redaction = %q, %v", doc.Editor.String(), err)
	}
}

func TestRedactRangesRetainsPhysicalMixedEndingsAndRejectsOverlaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("a\r\nb\rc"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.ClearHistory()
	before := doc.Editor.String()
	if changed, err := doc.RedactRanges([]text.Range{{Start: 0, End: 3}, {Start: 2, End: 5}}, false); err == nil || changed || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("overlap changed document: %q, %v", doc.Editor.String(), err)
	}
	if changed, err := doc.RedactRanges([]text.Range{{Start: 0, End: doc.Editor.RuneCount()}}, false); err != nil || !changed {
		t.Fatalf("mixed EOL redaction = %v, %v", changed, err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if output, err := os.ReadFile(path); err != nil || string(output) != "█\r\n█\r█" {
		t.Fatalf("redaction altered physical separators: %q, %v", output, err)
	}
}

func TestRedactRangesUsesRepresentableSymbolsInEveryOEMEncoding(t *testing.T) {
	for _, format := range []Encoding{DOS720, DOS737, DOS775, DOS857, DOS861, DOS869} {
		doc := New("oem")
		doc.Editor.Insert("A")
		doc.SetEncoding(format)
		if changed, err := doc.RedactRanges([]text.Range{{Start: 0, End: 1}}, false); err != nil || !changed || doc.Editor.String() != "#" {
			t.Fatalf("code page %d block redaction = %q, %v", format, doc.Editor.String(), err)
		}
		if _, err := format.EncodedLength(doc.Editor.String()); err != nil {
			t.Fatalf("code page %d block mask is not representable: %v", format, err)
		}
		if !doc.Editor.Undo() {
			t.Fatal("cannot undo OEM redaction")
		}
		if changed, err := doc.RedactRanges([]text.Range{{Start: 0, End: 1}}, true); err != nil || !changed || doc.Editor.String() != "." {
			t.Fatalf("code page %d Shift redaction = %q, %v", format, doc.Editor.String(), err)
		}
		if _, err := format.EncodedLength(doc.Editor.String()); err != nil {
			t.Fatalf("code page %d bullet mask is not representable: %v", format, err)
		}
	}
}
