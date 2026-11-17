package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestGB18030PUALiteralBytes(t *testing.T) {
	for _, sample := range []struct {
		value string
		data  []byte
	}{
		{"\ue000", []byte{0xaa, 0xa1}},
		{"\ue001", []byte{0xaa, 0xa2}},
		{"\ue4c5", []byte{0xfe, 0xfe}},
		{"\ue5e5", []byte{0xa3, 0xa0}},
		{"\ue7c7", []byte{0xa8, 0xbc}},
		{"\uf014", []byte{0x83, 0x38, 0x98, 0x37}},
		{"\uf8ff", []byte{0x84, 0x30, 0x81, 0x30}},
		{"\u1e3f", []byte{0x81, 0x35, 0xf4, 0x37}},
		{"\u3000", []byte{0xa1, 0xa1}},
		{"\U0001f600", []byte{0x94, 0x39, 0xfc, 0x36}},
		{"\U0010ffff", []byte{0xe3, 0x32, 0x9a, 0x35}},
	} {
		t.Run(sample.value, func(t *testing.T) {
			encoded, err := encodeText(sample.value, GB18030)
			if err != nil || !bytes.Equal(encoded, sample.data) {
				t.Errorf("GB18030 %U = %x, %v; want %x", []rune(sample.value), encoded, err, sample.data)
			}
			decoded, _, err := decodeLegacy(sample.data, GB18030)
			if err != nil || decoded != sample.value {
				t.Errorf("GB18030 %x = %U, %v; want %U", sample.data, []rune(decoded), err, []rune(sample.value))
			}
			if width, err := GB18030.EncodedLength(sample.value); err != nil || width != len(sample.data) {
				t.Errorf("byte length = %d, %v; want %d", width, err, len(sample.data))
			}
		})
	}
}

func TestGB18030PUADocumentMixedHistory(t *testing.T) {
	original := []byte("A\r\n\u4e2d\ue000\ue001\ue4c5\ue5e5\ue7c7\uf014\uf8ff\U0001f600\rB\n")
	want := []byte{'A', '\r', '\n', 0xd6, 0xd0, 0xaa, 0xa1, 0xaa, 0xa2, 0xfe, 0xfe, 0xa3, 0xa0, 0xa8, 0xbc, 0x83, 0x38, 0x98, 0x37, 0x84, 0x30, 0x81, 0x30, 0x94, 0x39, 0xfc, 0x36, '\r', 'B', '\n'}
	file, copyPath := filepath.Join(t.TempDir(), "source.txt"), filepath.Join(t.TempDir(), "copy.txt")
	if err := os.WriteFile(file, original, 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	editor, before, endings := doc.Editor, doc.Editor.String(), doc.EOLSequence()
	editor.MoveTo(editor.RuneCount(), false)
	editor.Insert("!")
	editor.DeleteBackward()
	if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
		t.Fatal("missing history fixture")
	}
	editor.SetSelection(text.Range{Start: 3, End: 1})
	selection, caret := editor.Selection(), editor.Caret()
	doc.SetEncoding(GB18030)
	if err := doc.ValidateEncoding(); err != nil {
		t.Fatal(err)
	}
	if err := doc.SaveCopy(copyPath); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(data, append(bytes.Clone(want), '!')) {
		t.Fatalf("copy=%x, %v", data, err)
	}
	if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("copy changed source=%x, %v", data, err)
	}
	if !doc.Dirty() || doc.SavedEncoding() != UTF8 || editor.AtSavePoint() {
		t.Fatal("copy accepted the pending source save")
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if doc.Editor != editor || editor.String() != before+"!" || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() || !editor.AtSavePoint() || doc.Dirty() || doc.EOLSequence() != endings {
		t.Fatal("save changed source, selection, history or mixed EOL")
	}
	loaded, err := OpenWithEncoding(file, GB18030)
	if err != nil || loaded.Editor.String() != before+"!" || loaded.Dirty() || loaded.EOLSequence() != endings {
		t.Fatalf("reopen=%+v, %v", loaded, err)
	}
	if err := loaded.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, append(bytes.Clone(want), '!')) {
		t.Fatalf("unchanged save=%x, %v", data, err)
	}
	if !editor.Undo() || editor.String() != before || !doc.Dirty() || !editor.Redo() || editor.String() != before+"!" || doc.Dirty() || !editor.Redo() || editor.String() != before || !doc.Dirty() || doc.EOLSequence() != endings {
		t.Fatal("save lost exact Undo/Redo boundary or mixed EOL")
	}
}
