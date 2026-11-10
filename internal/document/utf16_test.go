package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestUTF16LERoundTripAndConversionToUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unicode.txt")
	input := []byte{0xff, 0xfe, 'a', 0, '\r', 0, '\n', 0, 0x2d, 0x4e}
	if err := os.WriteFile(path, input, 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != "a\n中" || doc.EncodingLabel() != "UTF-16 LE" || doc.Dirty() {
		t.Fatalf("opened = %q encoding=%q dirty=%v", got, doc.EncodingLabel(), doc.Dirty())
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("文")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(output, append(input, 0x87, 0x65)) {
		t.Fatalf("round trip = %x, err=%v", output, err)
	}
	doc.SetEncoding(UTF8)
	if !doc.Dirty() {
		t.Fatal("encoding-only conversion did not mark dirty")
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	output, err = os.ReadFile(path)
	if err != nil || string(output) != "a\r\n中文" {
		t.Fatalf("UTF-8 conversion = %q, err=%v", output, err)
	}
}

func TestUTF16BEAndInvalidSurrogate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unicode.txt")
	if err := os.WriteFile(path, []byte{0xfe, 0xff, 0x4e, 0x2d}, 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil || doc.Editor.String() != "中" || doc.EncodingLabel() != "UTF-16 BE" {
		t.Fatalf("BE open = %+v, err=%v", doc, err)
	}
	if err := os.WriteFile(path, []byte{0xff, 0xfe, 0x00, 0xd8}, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("unpaired UTF-16 surrogate was accepted")
	}
}
