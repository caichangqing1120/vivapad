package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGB18030ExplicitOpenEditAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chinese.txt")
	original := []byte{0xd6, 0xd0, 0xce, 0xc4, '\r', '\n'}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("implicit UTF-8 open guessed a legacy encoding")
	}
	doc, err := OpenWithEncoding(path, GB18030)
	if err != nil || doc.Editor.String() != "中文\n" || doc.EncodingLabel() != "GB18030" || doc.Dirty() {
		t.Fatalf("explicit open: doc=%+v err=%v", doc, err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("unchanged round trip = %x, %v", got, err)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("本")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	want := append(append([]byte{}, original...), 0xb1, 0xbe)
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("edited round trip = %x, %v; want %x", got, err, want)
	}
}

func TestWindows1252UnrepresentableSaveDoesNotTouchDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "western.txt")
	original := []byte{'c', 'a', 'f', 0xe9, '\r', '\n'}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := OpenWithEncoding(path, Windows1252)
	if err != nil || doc.Editor.String() != "café\n" {
		t.Fatalf("windows-1252 open = %+v, %v", doc, err)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("中")
	if err := doc.Save(""); err == nil {
		t.Fatal("unrepresentable text was saved with replacement characters")
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) || !doc.Dirty() {
		t.Fatalf("failed save changed disk=%x or dirty=%v, err=%v", got, doc.Dirty(), err)
	}
}

func TestExplicitLegacyOpenRejectsInvalidAndBinaryBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.txt")
	for _, data := range [][]byte{{'a', 0, 'b'}, {0x81, 0x30, 0x81, 0xff}} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenWithEncoding(path, GB18030); err == nil {
			t.Fatalf("unsafe GB18030 byte sequence accepted: %x", data)
		}
	}
}

func TestConvertingToGB18030MarksDirtyAndSavesEncodedBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "convert.txt")
	if err := os.WriteFile(path, []byte("中文"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.SetEncoding(GB18030)
	if !doc.Dirty() {
		t.Fatal("encoding-only change did not mark the document dirty")
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, []byte{0xd6, 0xd0, 0xce, 0xc4}) {
		t.Fatalf("converted bytes = %x, %v", got, err)
	}
}
