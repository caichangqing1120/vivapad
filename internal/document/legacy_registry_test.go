package document

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyRegistryCoversSupportedOriginalCodePages(t *testing.T) {
	if UTF8 != 0 || UTF8BOM != 1 || UTF16LE != 2 || UTF16BE != 3 || GB18030 != 4 || Windows1252 != 5 {
		t.Fatal("existing persisted encoding identifiers changed")
	}
	specs := LegacyEncodings()
	if len(specs) != 47 {
		t.Fatalf("supported explicit encodings = %d, want 46 original character sets plus GB18030", len(specs))
	}
	seenCodes, seenNames := map[int]bool{}, map[string]bool{}
	for _, spec := range specs {
		if !spec.Encoding.IsLegacy() || spec.CodePage <= 0 || spec.Label == "" || spec.Group == "" ||
			seenCodes[spec.CodePage] || seenNames[spec.Label] {
			t.Fatalf("invalid or repeated legacy registry entry %+v", spec)
		}
		seenCodes[spec.CodePage], seenNames[spec.Label] = true, true
		if got, ok := EncodingFromCodePage(spec.CodePage); !ok || got != spec.Encoding {
			t.Fatalf("code page %d maps to %d, %v", spec.CodePage, got, ok)
		}
		if got, ok := EncodingFromLabel(spec.Label); !ok || got != spec.Encoding {
			t.Fatalf("label %q maps to %d, %v", spec.Label, got, ok)
		}
	}
	for _, test := range []struct {
		code int
		want Encoding
	}{
		{54936, GB18030}, {1252, Windows1252}, {1251, Windows1251}, {932, ShiftJIS},
		{936, GBK}, {950, Big5}, {949, Windows949}, {51949, EUCKR}, {437, DOS437},
	} {
		if got, ok := EncodingFromCodePage(test.code); !ok || got != test.want {
			t.Fatalf("code page %d = %d, %v; want %d", test.code, got, ok, test.want)
		}
	}
	for _, supported := range []int{720, 737, 775, 857, 861, 869} {
		if got, ok := EncodingFromCodePage(supported); !ok || !got.IsLegacy() {
			t.Fatalf("OEM code page %d maps to %d, %v", supported, got, ok)
		}
	}
}

func TestNewLegacyCodecsOpenAndSaveWithoutChangingOriginalBytes(t *testing.T) {
	for _, test := range []struct {
		name   string
		format Encoding
		bytes  []byte
		text   string
	}{
		{"Cyrillic Windows-1251", Windows1251, []byte{0xcf, 0xf0, 0xe8, 0xe2, 0xe5, 0xf2}, "Привет"},
		{"Japanese Shift-JIS", ShiftJIS, []byte{0x93, 0xfa, 0x96, 0x7b}, "日本"},
		{"Chinese GBK", GBK, []byte{0xd6, 0xd0, 0xce, 0xc4}, "中文"},
		{"Western ISO-8859-1", ISO8859_1, []byte{'c', 'a', 'f', 0xe9}, "café"},
		{"OEM 437", DOS437, []byte{'c', 'a', 'f', 0x82}, "café"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "legacy.txt")
			if err := os.WriteFile(path, test.bytes, 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := OpenWithEncoding(path, test.format)
			if err != nil || doc.Editor.String() != test.text || doc.Encoding() != test.format || doc.Dirty() {
				t.Fatalf("legacy open = %+v, %v", doc, err)
			}
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != string(test.bytes) {
				t.Fatalf("legacy round-trip = %x, %v", data, err)
			}
		})
	}
}

func TestNewLegacyCodecsRejectMalformedInputAndUnmappableSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.txt")
	if err := os.WriteFile(path, []byte{0x82}, 0600); err != nil {
		t.Fatal(err)
	}
	if doc, err := OpenWithEncoding(path, ShiftJIS); err == nil || doc != nil {
		t.Fatalf("accepted truncated Shift-JIS lead byte: %+v, %v", doc, err)
	}
	if err := os.WriteFile(path, []byte{'A'}, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := OpenWithEncoding(path, ISO8859_1)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert("猫")
	if err := doc.Save(""); err == nil {
		t.Fatal("unmappable edit overwrote an ISO-8859-1 file")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "A" {
		t.Fatalf("failed save changed bytes to %x, %v", data, err)
	}
	if got := doc.Encoding(); got != ISO8859_1 || !doc.Dirty() {
		t.Fatalf("failed save lost document state: encoding=%v dirty=%v", got, doc.Dirty())
	}
}

func TestEncodedLengthRejectsUnknownCharacterSet(t *testing.T) {
	if _, err := Encoding(255).EncodedLength("text"); err == nil {
		t.Fatal("unknown encoding returned a plausible byte length")
	}
	if got, err := ShiftJIS.EncodedLength("日本"); err != nil || got != 4 {
		t.Fatalf("Shift-JIS byte length = %d, %v", got, err)
	}
}
