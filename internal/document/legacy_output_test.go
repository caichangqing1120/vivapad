package document

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestTextOutputRejectsNULBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		format Encoding
	}{
		{"UTF-8", UTF8}, {"UTF-8 BOM", UTF8BOM}, {"UTF-16 LE", UTF16LE}, {"UTF-16 BE", UTF16BE},
		{"Windows-1252", Windows1252}, {"GB18030", GB18030},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source, copyPath := filepath.Join(root, "source.txt"), filepath.Join(root, "copy.txt")
			const saved, existingCopy = "first\r\nsecond\nlast", "existing copy\r\n"
			for name, value := range map[string]string{source: saved, copyPath: existingCopy} {
				if err := os.WriteFile(name, []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			doc, err := Open(source)
			if err != nil {
				t.Fatal(err)
			}
			doc.SetEncoding(tc.format)
			editor := doc.Editor
			editor.MoveTo(editor.RuneCount(), false)
			editor.Insert("\x00")
			editor.BreakUndoGroup()
			editor.Insert("x")
			if !editor.Undo() || !strings.ContainsRune(editor.String(), 0) || !editor.CanUndo() || !editor.CanRedo() {
				t.Fatal("fixture must contain a NUL and Undo/Redo history")
			}
			editor.SetSelection(text.Range{Start: 3, End: 1})
			before, selection, caret, endings := editor.String(), editor.Selection(), editor.Caret(), doc.EOLSequence()
			for _, op := range []struct {
				name string
				run  func() error
			}{
				{"validation", doc.ValidateEncoding},
				{"length", func() error { _, err := tc.format.EncodedLength(before); return err }},
				{"Save Copy", func() error { return doc.SaveCopy(copyPath) }},
				{"Save", func() error { return doc.Save("") }},
			} {
				if err := op.run(); err == nil || !strings.Contains(err.Error(), "NUL") {
					t.Errorf("%s must refuse NUL before writing, got %v", op.name, err)
				}
			}
			if editor.String() != before || editor.Selection() != selection || editor.Caret() != caret || doc.EOLSequence() != endings ||
				!editor.CanUndo() || !editor.CanRedo() || editor.AtSavePoint() || !doc.Dirty() || doc.Encoding() != tc.format || doc.SavedEncoding() != UTF8 {
				t.Error("refusal changed editor text, format, selection, history, or saved boundary")
			}
			for name, value := range map[string]string{source: saved, copyPath: existingCopy} {
				if data, err := os.ReadFile(name); err != nil || string(data) != value {
					t.Errorf("%s changed after refusal: %q, %v", filepath.Base(name), data, err)
				}
			}
		})
	}
}

func TestLegacyUnrepresentableOutputRejectedBeforeWrite(t *testing.T) {
	for _, value := range []string{"\u4e2d", "\U0001f600"} {
		for _, operation := range []string{"validate", "length", "save", "copy"} {
			t.Run(value+"/"+operation, func(t *testing.T) {
				root := t.TempDir()
				file, target := filepath.Join(root, "source.txt"), filepath.Join(root, "copy.txt")
				original := []byte("A\r\n" + value + "\nB\r")
				if err := os.WriteFile(file, original, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte("existing destination"), 0o600); err != nil {
					t.Fatal(err)
				}
				doc, err := Open(file)
				if err != nil {
					t.Fatal(err)
				}
				editor, before, savedPath := doc.Editor, doc.Editor.String(), doc.Path
				editor.MoveTo(editor.RuneCount(), false)
				editor.Insert("!")
				editor.DeleteBackward()
				if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
					t.Fatal("fixture did not establish Undo and Redo")
				}
				editor.SetSelection(text.Range{Start: 3, End: 1})
				selection, caret, endings := editor.Selection(), editor.Caret(), doc.EOLSequence()
				doc.SetEncoding(Windows1252)
				switch operation {
				case "validate":
					err = doc.ValidateEncoding()
				case "length":
					_, err = Windows1252.EncodedLength(editor.String())
				case "save":
					err = doc.Save("")
				case "copy":
					err = doc.SaveCopy(target)
				}
				if err == nil {
					t.Errorf("%s accepted unrepresentable Windows-1252 text", operation)
				}
				if doc.Editor != editor || editor.String() != before+"!" || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() || editor.AtSavePoint() || !doc.Dirty() || doc.Path != savedPath || doc.Encoding() != Windows1252 || doc.SavedEncoding() != UTF8 || doc.EOLSequence() != endings {
					t.Error("lossy output failure changed source, format, history or line endings")
				}
				if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, original) {
					t.Errorf("lossy output changed source disk bytes: %x, %v", data, err)
				}
				if data, err := os.ReadFile(target); err != nil || string(data) != "existing destination" {
					t.Errorf("lossy output changed destination bytes: %x, %v", data, err)
				}
				if !editor.Undo() || editor.String() != before || !editor.AtSavePoint() || !doc.Dirty() || !editor.Redo() || !editor.Redo() || editor.String() != before || editor.AtSavePoint() || !doc.Dirty() {
					t.Error("lossy output failure changed the old saved boundary or history ordering")
				}
			})
		}
	}
}

func TestLegacyOutputKeepsLiteralSupplementaryBytes(t *testing.T) {
	file := filepath.Join(t.TempDir(), "unicode.txt")
	if err := os.WriteFile(file, []byte("\u4e2d\U0001f600\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	doc.SetEncoding(GB18030)
	if err := doc.ValidateEncoding(); err != nil {
		t.Fatal(err)
	}
	if width, err := GB18030.EncodedLength("\u4e2d\U0001f600\r\n"); err != nil || width != 8 {
		t.Fatalf("lossless GB18030 byte length = %d, %v", width, err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	want := []byte{0xd6, 0xd0, 0x94, 0x39, 0xfc, 0x36, 0x0d, 0x0a}
	if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, want) {
		t.Fatalf("lossless GB18030 output = %x, %v; want %x", data, err, want)
	}
	loaded, err := OpenWithEncoding(file, GB18030)
	if err != nil || loaded.Editor.String() != "\u4e2d\U0001f600\n" || loaded.Dirty() {
		t.Fatalf("lossless output did not reopen unchanged: %+v, %v", loaded, err)
	}
}

func TestLegacyOutputNeverChangesAcceptedBMPText(t *testing.T) {
	for value := rune(1); value <= 0xffff; value++ {
		if value >= 0xd800 && value <= 0xdfff {
			continue
		}
		original := string(value)
		data, err := encodeText(original, GB18030)
		if err != nil {
			continue
		}
		decoded, _, err := decodeLegacy(data, GB18030)
		if err != nil || decoded != original {
			t.Fatalf("accepted GB18030 %U became %U via %x: %v", value, []rune(decoded), data, err)
		}
	}
}
