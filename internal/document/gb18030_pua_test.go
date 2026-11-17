package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestGB18030PUALiteralsAndExistingBytes(t *testing.T) {
	for _, sample := range []struct {
		value string
		bytes []byte
	}{
		{"\uE000", []byte{0xaa, 0xa1}},
		{"\uE001", []byte{0xaa, 0xa2}},
		{"\uE4C5", []byte{0xfe, 0xfe}},
		{"\uE5E5", []byte{0xa3, 0xa0}},
		{"\uE7C7", []byte{0xa8, 0xbc}},
		{"\u3000", []byte{0xa1, 0xa1}},
		{"\uF014", []byte{0x83, 0x38, 0x98, 0x37}},
		{"\U0001F600", []byte{0x94, 0x39, 0xfc, 0x36}},
	} {
		t.Run(sample.value, func(t *testing.T) {
			encoded, err := encodeText(sample.value, GB18030)
			if err != nil || !bytes.Equal(encoded, sample.bytes) {
				t.Fatalf("encode %U = %x, %v; want %x", []rune(sample.value), encoded, err, sample.bytes)
			}
			decoded, _, err := decodeLegacy(sample.bytes, GB18030)
			if err != nil || decoded != sample.value {
				t.Fatalf("decode %x = %U, %v; want %U", sample.bytes, []rune(decoded), err, []rune(sample.value))
			}
			if length, err := GB18030.EncodedLength(sample.value); err != nil || length != len(sample.bytes) {
				t.Fatalf("encoded length = %d, %v", length, err)
			}
		})
	}
}

func TestGB18030PUAOpenSaveCopyAndHistory(t *testing.T) {
	root := t.TempDir()
	source, copyPath := filepath.Join(root, "source.txt"), filepath.Join(root, "copy.txt")
	original := []byte("A\uE000\uE5E5\r\n")
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("!")
	doc.Editor.SetSelection(text.Range{Start: 3, End: 1})
	selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
	doc.SetEncoding(GB18030)
	if err := doc.ValidateEncoding(); err != nil {
		t.Fatal(err)
	}
	if err := doc.SaveCopy(copyPath); err != nil {
		t.Fatal(err)
	}
	want := []byte{'A', 0xaa, 0xa1, 0xa3, 0xa0, '\r', '\n', '!'}
	if disk, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(disk, want) || !doc.Dirty() || doc.SavedEncoding() != UTF8 {
		t.Fatalf("copy bytes/state = %x/%v, %v", disk, doc.Dirty(), err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if disk, err := os.ReadFile(source); err != nil || !bytes.Equal(disk, want) || doc.Dirty() || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret {
		t.Fatalf("save bytes/state = %x/%v, %v", disk, doc.Dirty(), err)
	}
	reopened, err := OpenWithEncoding(source, GB18030)
	if err != nil || reopened.Editor.String() != "A\uE000\uE5E5\n!" {
		t.Fatalf("reopen = %+v, %v", reopened, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "A\uE000\uE5E5\n" || !doc.Dirty() || !doc.Editor.Redo() || doc.Dirty() {
		t.Fatal("Save lost Undo/Redo or saved boundary")
	}
}
