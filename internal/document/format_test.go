package document

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatOnlyChangesAreDirtyAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "format.txt")
	if err := os.WriteFile(path, []byte("one\ntwo"), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.SetLineEnding(CRLF)
	doc.SetUTF8BOM(true)
	if !doc.Dirty() || doc.Editor.String() != "one\ntwo" || doc.NewlineLabel() != "Windows (CR LF)" {
		t.Fatalf("format state = dirty:%v, value:%q, eol:%q", doc.Dirty(), doc.Editor.String(), doc.NewlineLabel())
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if output, err := os.ReadFile(path); err != nil || string(output) != "\xef\xbb\xbfone\r\ntwo" || doc.Dirty() {
		t.Fatalf("saved = %q, err=%v, dirty=%v", output, err, doc.Dirty())
	}
	doc.SetLineEnding(LF)
	doc.SetUTF8BOM(false)
	if !doc.Dirty() {
		t.Fatal("format change after save was not dirty")
	}
}

func TestRevertingFormatChoiceClearsDirtyFlag(t *testing.T) {
	doc := New("new 1")
	doc.SetLineEnding(CRLF)
	if !doc.Dirty() {
		t.Fatal("new format should be dirty")
	}
	doc.SetLineEnding(LF)
	if doc.Dirty() {
		t.Fatal("reverting format still dirty")
	}
}

func TestOpenAndSaveMacintoshCROnlyPreservesFinalNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mac.txt")
	original := "one\rtwo\r"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.LineEnding() != CR || doc.NewlineLabel() != "Macintosh (CR)" || doc.Editor.String() != "one\ntwo\n" || doc.Dirty() {
		t.Fatalf("CR load = %q ending=%v dirty=%v", doc.Editor.String(), doc.LineEnding(), doc.Dirty())
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != original {
		t.Fatalf("CR round-trip = %q, %v", data, err)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("three")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "one\rtwo\rthree" {
		t.Fatalf("edited CR save = %q, %v", data, err)
	}
}

func TestCRFormatOnlyChangesDirtyAndRevert(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("a\nb")
	doc.SetLineEnding(CR)
	if !doc.Dirty() || doc.Editor.String() != "a\nb" {
		t.Fatal("CR conversion did not mark format-only change dirty")
	}
	doc.Editor.ClearHistory()
	doc.SetLineEnding(LF)
	if doc.Dirty() != true || doc.Editor.CanUndo() {
		t.Fatal("format-only transition should not add text undo history")
	}
	path := filepath.Join(t.TempDir(), "mac.txt")
	if err := os.WriteFile(path, []byte("a\rb"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.SetLineEnding(CRLF)
	if !doc.Dirty() {
		t.Fatal("changing CR to CRLF should mark dirty")
	}
	doc.SetLineEnding(CR)
	if doc.Dirty() {
		t.Fatal("returning to original CR should clear dirty")
	}
}

func TestCRLineEndingsRoundTripUTF16AndLegacyEncoding(t *testing.T) {
	root := t.TempDir()
	utf16Path := filepath.Join(root, "utf16.txt")
	utf16Doc := New("utf16.txt")
	utf16Doc.Editor.Insert("猫\n犬\n")
	utf16Doc.SetEncoding(UTF16LE)
	utf16Doc.SetLineEnding(CR)
	if err := utf16Doc.Save(utf16Path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Open(utf16Path)
	if err != nil || loaded.LineEnding() != CR || loaded.Editor.String() != "猫\n犬\n" || loaded.Encoding() != UTF16LE {
		t.Fatalf("UTF-16 CR round-trip = %+v, %v", loaded, err)
	}
	legacyPath := filepath.Join(root, "legacy.txt")
	if err := os.WriteFile(legacyPath, []byte("caf\xe9\rfin\r"), 0600); err != nil {
		t.Fatal(err)
	}
	legacy, err := OpenWithEncoding(legacyPath, Windows1252)
	if err != nil || legacy.LineEnding() != CR || legacy.Editor.String() != "café\nfin\n" {
		t.Fatalf("legacy CR load = %+v, %v", legacy, err)
	}
	copyPath := filepath.Join(root, "copy.txt")
	if err := legacy.SaveCopy(copyPath); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(copyPath); err != nil || string(data) != "caf\xe9\rfin\r" {
		t.Fatalf("legacy CR copy = %q, %v", data, err)
	}
}

func TestMixedEOLWithCRNormalizesForEditingAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	for _, value := range []string{"a\r\nb\r", "a\rb\n", "a\nb\r\n"} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		doc, err := Open(path)
		if err != nil || !doc.MixedLineEndings() || doc.Editor.String() != "a\nb\n" {
			t.Fatalf("mixed line endings %q loaded as %+v: %v", value, doc, err)
		}
		if err := doc.Save(""); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != value {
			t.Fatalf("mixed line endings changed on save: %q, %v", data, err)
		}
	}
}
