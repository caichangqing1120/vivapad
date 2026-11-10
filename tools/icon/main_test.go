package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestPackagedIconRetainsEveryOriginalPNGRendition(t *testing.T) {
	sourceDir := filepath.Join("..", "..", "assets")
	encoded, err := originalICNS(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded[:4]) != "icns" || int(binary.BigEndian.Uint32(encoded[4:8])) != len(encoded) {
		t.Fatal("invalid ICNS container")
	}
	want := map[string]string{
		"ic11": "vivapad-icon-32.png",
		"ic12": "vivapad-icon-64.png",
		"ic07": "vivapad-icon-128.png",
		"ic10": "vivapad-icon.png",
	}
	for offset := 8; offset < len(encoded); {
		length := int(binary.BigEndian.Uint32(encoded[offset+4 : offset+8]))
		if length < 8 || offset+length > len(encoded) {
			t.Fatalf("invalid ICNS entry length %d at %d", length, offset)
		}
		kind := string(encoded[offset : offset+4])
		if path, ok := want[kind]; ok {
			original, err := os.ReadFile(filepath.Join(sourceDir, path))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded[offset+8:offset+length], original) {
				t.Fatalf("%s must retain the original PNG without resampling", kind)
			}
			delete(want, kind)
		}
		offset += length
	}
	if len(want) != 0 {
		t.Fatalf("missing original renditions: %v", want)
	}
}
