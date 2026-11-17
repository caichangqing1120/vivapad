package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestGB18030PUAMappingMatchesFrozenDigest(t *testing.T) {
	digest, count := sha256.New(), 0
	for lead := 0x81; lead <= 0xfe; lead++ {
		for trail := 0x40; trail <= 0xfe; trail++ {
			if trail == 0x7f {
				continue
			}
			data := []byte{byte(lead), byte(trail)}
			decoded, _, err := decodeLegacy(data, GB18030)
			if err != nil {
				t.Fatalf("valid old-standard pair %x: %v", data, err)
			}
			values := []rune(decoded)
			if len(values) != 1 {
				t.Fatalf("pair %x decoded as %U", data, values)
			}
			if values[0] < 0xe000 || values[0] > 0xf8ff {
				continue
			}
			digest.Write(data)
			var scalar [4]byte
			binary.BigEndian.PutUint32(scalar[:], uint32(values[0]))
			digest.Write(scalar[:])
			count++
		}
	}
	if got := fmt.Sprintf("%x", digest.Sum(nil)); count != 2068 || got != "e47898ee197ed3f428e0dc76ce300a070126e7301b5d1491b0d3c0262d623af2" {
		t.Fatalf("PUA mappings=%d digest=%s", count, got)
	}
}

func TestGB18030AllBMPScalarsAndExistingBytes(t *testing.T) {
	digest, previouslyAccepted := sha256.New(), 0
	for value := rune(1); value <= 0xffff; value++ {
		if value >= 0xd800 && value <= 0xdfff {
			continue
		}
		input := []byte(string(value))
		encoded, err := encodeText(string(value), GB18030)
		if err != nil {
			t.Fatalf("valid scalar %U: %v", value, err)
		}
		decoded, _, err := decodeLegacy(encoded, GB18030)
		if err != nil || decoded != string(value) {
			t.Fatalf("%U via %x became %U: %v", value, encoded, []rune(decoded), err)
		}
		old, oldErr := simplifiedchinese.GB18030.NewEncoder().Bytes(input)
		oldText, decodeErr := simplifiedchinese.GB18030.NewDecoder().Bytes(old)
		if oldErr != nil || decodeErr != nil || !bytes.Equal(oldText, input) {
			continue
		}
		if !bytes.Equal(encoded, old) {
			t.Fatalf("existing %U changed %x -> %x", value, old, encoded)
		}
		var scalar [4]byte
		binary.BigEndian.PutUint32(scalar[:], uint32(value))
		digest.Write(scalar[:])
		digest.Write(encoded)
		previouslyAccepted++
	}
	if got := fmt.Sprintf("%x", digest.Sum(nil)); previouslyAccepted != 61419 || got != "15888d47176305b44e937a49cfa6bd2adf4059f648211ad221ae9e4056c73c8b" {
		t.Fatalf("existing BMP=%d digest=%s", previouslyAccepted, got)
	}
}

func TestGB18030UnitBoundariesAndRefusal(t *testing.T) {
	for _, sample := range []struct {
		data  []byte
		value string
	}{
		{[]byte{}, ""},
		{[]byte{0x81, 0xaa, 0xa1, 0x40}, "\u4efe\ue4c6"},
		{[]byte{0xa1, 0xa1, 0x81, 0xaa, 0xa1, 0x40, 0xaa, 0xa1}, "\u3000\u4efe\ue4c6\ue000"},
		{[]byte{0x81, 0x30, 0x81, 0x30, 0xa8, 0xbc, 0x83, 0x38, 0x98, 0x37}, "\u0080\ue7c7\uf014"},
		{[]byte{0x84, 0x31, 0xa4, 0x37}, "\ufffd"},
	} {
		decoded, _, err := decodeLegacy(sample.data, GB18030)
		if err != nil || decoded != sample.value {
			t.Errorf("unit-boundary %x = %U, %v; want %U", sample.data, []rune(decoded), err, []rune(sample.value))
		}
		encoded, err := encodeText(sample.value, GB18030)
		if err != nil || !bytes.Equal(encoded, sample.data) {
			t.Errorf("unit-boundary output=%x, %v; want %x", encoded, err, sample.data)
		}
	}
	for _, data := range [][]byte{{0}, {0xaa, 0xa1, 0}, {0x80}, {0xff}, {0x81}, {0x81, 0x30}, {0x81, 0x30, 0x81}, {0x81, 0x3a, 0x81, 0x30}, {0x81, 0x30, 0x80, 0x30}, {0x81, 0x30, 0x81, 0x3a}, {0xa1, 0x7f}} {
		if value, _, err := decodeLegacy(data, GB18030); err == nil {
			t.Errorf("malformed/noncanonical %x accepted as %U", data, []rune(value))
		}
	}
	for _, input := range [][]byte{{0xff}, {0xc0, 0xaf}, {0xed, 0xa0, 0x80}, {0xee, 0x80}} {
		if data, err := encodeText(string(input), GB18030); err == nil {
			t.Errorf("invalid UTF-8 %x accepted as %x", input, data)
		}
	}
}

func TestLegacyUnicodeLossGuardWithActualBrokenBackend(t *testing.T) {
	for _, value := range []string{"\ue000", "\ue001"} {
		input := []byte(value)
		codec := simplifiedchinese.GB18030
		data, err := codec.NewEncoder().Bytes(input)
		if err != nil {
			t.Fatal("fixture must reproduce a successful lossy backend conversion")
		}
		decoded, err := codec.NewDecoder().Bytes(data)
		if err != nil || bytes.Equal(decoded, input) {
			t.Fatal("fixture no longer reproduces the pinned backend defect")
		}
		if output, err := encodeLegacyChecked(input, "GB18030", codec.NewEncoder().Bytes, codec.NewDecoder().Bytes); err == nil || output != nil {
			t.Fatalf("application Unicode-loss guard accepted %x -> %x", input, output)
		}
	}
}

func TestGB18030PUAConcurrentConversions(t *testing.T) {
	const value = "\u4e2d\ue000\uf014\ue5e5\U0001f600"
	want := []byte{0xd6, 0xd0, 0xaa, 0xa1, 0x83, 0x38, 0x98, 0x37, 0xa3, 0xa0, 0x94, 0x39, 0xfc, 0x36}
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			for range 100 {
				data, err := encodeText(value, GB18030)
				if err != nil || !bytes.Equal(data, want) {
					t.Errorf("concurrent output=%x, %v", data, err)
					return
				}
				decoded, _, err := decodeLegacy(data, GB18030)
				if err != nil || decoded != value {
					t.Errorf("concurrent decode=%U, %v", []rune(decoded), err)
					return
				}
			}
		})
	}
	workers.Wait()
}
