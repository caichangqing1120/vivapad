package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/transform"
)

func TestOEMCodePagesRoundTripSourceBytesAndRejectMissingGlyphs(t *testing.T) {
	for _, sample := range []struct {
		format   Encoding
		codePage int
		value    byte
		text     string
	}{
		{DOS720, 720, 0xa0, "ب"},
		{DOS737, 737, 0x80, "Α"},
		{DOS775, 775, 0x80, "Ć"},
		{DOS857, 857, 0xa7, "ğ"},
		{DOS861, 861, 0x8d, "Þ"},
		{DOS869, 869, 0xd5, "Ω"},
	} {
		t.Run(sample.text, func(t *testing.T) {
			format, ok := EncodingFromCodePage(sample.codePage)
			if !ok || format != sample.format {
				t.Fatalf("code page %d maps to %d, %v", sample.codePage, format, ok)
			}
			encoded, err := format.EncodedLength("A" + sample.text)
			if err != nil || encoded != 2 {
				t.Fatalf("encoded length = %d, %v", encoded, err)
			}
			path := filepath.Join(t.TempDir(), "oem.txt")
			original := []byte{'A', sample.value}
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := OpenWithEncoding(path, format)
			if err != nil || doc.Editor.String() != "A"+sample.text || doc.Dirty() {
				t.Fatalf("decoded OEM document = %+v, %v", doc, err)
			}
			doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
			doc.Editor.Insert("!")
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			if output, err := os.ReadFile(path); err != nil || !bytes.Equal(output, []byte{'A', sample.value, '!'}) {
				t.Fatalf("OEM saved bytes = %x, %v", output, err)
			}
			doc.Editor.Insert("🙂")
			if err := doc.Save(""); err == nil {
				t.Fatal("unmappable emoji overwrote the OEM file")
			}
			if output, err := os.ReadFile(path); err != nil || !bytes.Equal(output, []byte{'A', sample.value, '!'}) {
				t.Fatalf("failed save changed OEM bytes = %x, %v", output, err)
			}
		})
	}
}

func TestOEMCodePageTablesRejectUndefinedBytesAndRoundTripMappedBytes(t *testing.T) {
	for _, format := range []Encoding{DOS720, DOS737, DOS775, DOS857, DOS861, DOS869} {
		codec := legacyCodec(format)
		if codec == nil {
			t.Fatalf("missing OEM codec %d", format)
		}
		for value := range 256 {
			decoded, err := codec.NewDecoder().Bytes([]byte{byte(value)})
			if err != nil {
				if format != DOS857 && format != DOS869 {
					t.Fatalf("code page %d byte %02x unexpectedly undefined: %v", format, value, err)
				}
				continue
			}
			encoded, err := codec.NewEncoder().Bytes(decoded)
			if err != nil || !bytes.Equal(encoded, []byte{byte(value)}) {
				t.Fatalf("code page %d byte %02x round-trip = %x, %v", format, value, encoded, err)
			}
		}
	}
	for _, value := range []byte{0xd5, 0xe7, 0xf2} {
		if _, err := legacyCodec(DOS857).NewDecoder().Bytes([]byte{value}); err == nil {
			t.Fatalf("undefined CP857 byte %02x accepted", value)
		}
	}
	for _, value := range []byte{0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x87, 0x93, 0x94} {
		if _, err := legacyCodec(DOS869).NewDecoder().Bytes([]byte{value}); err == nil {
			t.Fatalf("undefined CP869 byte %02x accepted", value)
		}
	}
}

func TestOEMCodecStreamingBoundaries(t *testing.T) {
	codec := legacyCodec(DOS720)
	encodedArabic := []byte("ب")
	if written, consumed, err := codec.NewDecoder().Transform(make([]byte, 1), []byte{0xa0}, true); written != 0 || consumed != 0 || err != transform.ErrShortDst {
		t.Fatalf("small decode buffer = %d,%d,%v", written, consumed, err)
	}
	decoded := make([]byte, len(encodedArabic))
	if written, consumed, err := codec.NewDecoder().Transform(decoded, []byte{0xa0}, true); written != len(encodedArabic) || consumed != 1 || err != nil || !bytes.Equal(decoded, encodedArabic) {
		t.Fatalf("full decode buffer = %d,%d,%v,%x", written, consumed, err, decoded)
	}
	if written, consumed, err := codec.NewEncoder().Transform(make([]byte, 1), encodedArabic[:1], false); written != 0 || consumed != 0 || err != transform.ErrShortSrc {
		t.Fatalf("partial UTF-8 = %d,%d,%v", written, consumed, err)
	}
	if _, _, err := codec.NewEncoder().Transform(make([]byte, 1), encodedArabic[:1], true); err == nil {
		t.Fatal("truncated UTF-8 at EOF encoded")
	}
	if written, consumed, err := codec.NewEncoder().Transform(nil, encodedArabic, true); written != 0 || consumed != 0 || err != transform.ErrShortDst {
		t.Fatalf("small encode buffer = %d,%d,%v", written, consumed, err)
	}
	if _, err := codec.NewEncoder().Bytes([]byte{0xff}); err == nil {
		t.Fatal("invalid UTF-8 encoded")
	}
}
