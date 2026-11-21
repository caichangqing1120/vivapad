package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNFOOpenDecodesOEM437AndPreservesMixedLineEndings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "poster.NFO")
	original := []byte{0xda, 0xc4, 0xbf, '\r', '\n', 0xb3, 0x82, 0xb3, '\n', 0xc0, 0xc4, 0xd9}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	const text = "\u250c\u2500\u2510\n\u2502\u00e9\u2502\n\u2514\u2500\u2518"
	if doc.Editor.String() != text || doc.Encoding() != DOS437 || doc.Dirty() {
		t.Fatalf("opened = %q, encoding %v, dirty %v", doc.Editor.String(), doc.Encoding(), doc.Dirty())
	}
	source, err := ReadSearchSource(path, UTF8, 4096)
	if err != nil || source.Text != text || source.Encoding != DOS437 {
		t.Fatalf("search source = %+v, %v", source, err)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("!")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(path); err != nil || !bytes.Equal(saved, append(original, '!')) {
		t.Fatalf("saved OEM bytes = %x, %v", saved, err)
	}
}

func TestNFOOpenPreservesUnicodeAndExplicitEncoding(t *testing.T) {
	for _, sample := range []struct {
		name     string
		data     []byte
		encoding Encoding
		text     string
	}{
		{"utf8.nfo", []byte("\u2500\r\n"), UTF8, "\u2500\n"},
		{"bom.nfo", []byte("\xef\xbb\xbf\u2500"), UTF8BOM, "\u2500"},
		{"utf16.nfo", []byte{0xff, 0xfe, 0, 0x25}, UTF16LE, "\u2500"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), sample.name)
			if err := os.WriteFile(path, sample.data, 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := Open(path)
			if err != nil || doc.Encoding() != sample.encoding || doc.Editor.String() != sample.text {
				t.Fatalf("open = %+v, %v", doc, err)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "explicit.nfo")
	if err := os.WriteFile(path, []byte{0x82}, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := OpenWithEncoding(path, Windows1252)
	if err != nil || doc.Encoding() != Windows1252 || doc.Editor.String() != "\u201a" {
		t.Fatalf("explicit open = %+v, %v", doc, err)
	}
}

func TestNFOFallbackRejectsNULAndInvalidBOMAndDoesNotApplyToOtherFiles(t *testing.T) {
	for _, sample := range []struct {
		name string
		data []byte
	}{
		{"binary.nfo", []byte{0, 0xc4}},
		{"utf8.nfo", []byte{0xef, 0xbb, 0xbf, 0xc4}},
		{"utf16.nfo", []byte{0xff, 0xfe, 0xc4}},
		{"other.txt", []byte{0xc4}},
	} {
		path := filepath.Join(t.TempDir(), sample.name)
		if err := os.WriteFile(path, sample.data, 0600); err != nil {
			t.Fatal(err)
		}
		if doc, err := Open(path); err == nil || doc != nil {
			t.Errorf("accepted invalid input %s", sample.name)
		}
	}
}
