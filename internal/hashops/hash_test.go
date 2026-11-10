package hashops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDigestKnownVectorsAndInvalidAlgorithm(t *testing.T) {
	for _, test := range []struct {
		algorithm Algorithm
		want      string
	}{
		{MD5, "900150983cd24fb0d6963f7d28e17f72"},
		{SHA1, "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{SHA256, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{SHA512, "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
			"2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
	} {
		got, err := Digest(test.algorithm, []byte("abc"))
		if err != nil || got != test.want {
			t.Fatalf("algorithm %d = %q, %v; want %q", test.algorithm, got, err, test.want)
		}
	}
	if got, err := Digest(Algorithm(99), []byte("abc")); err == nil || got != "" {
		t.Fatalf("invalid hash algorithm = %q, %v", got, err)
	}
}

func TestTextHashUsesUTF8AndRetainsBlankLinesInPerLineMode(t *testing.T) {
	got, err := Text(SHA256, "猫", false)
	if err != nil || got != "a810bf845a307382118e6e6aef21a79f6a26b1b3ad2225fe97b7ee4c125e3380" {
		t.Fatalf("UTF-8 text digest = %q, %v", got, err)
	}
	got, err = Text(MD5, "a\n\nb\n", true)
	if err != nil || got != "0cc175b9c0f1b6a831c399e269772661\r\n\r\n92eb5ffee6ae2fec3ad71c777531578f\r\n" {
		t.Fatalf("per-line digest = %q, %v", got, err)
	}
	if got, err := Text(MD5, "", true); err != nil || got != "" {
		t.Fatalf("empty per-line digest = %q, %v", got, err)
	}
}

func TestFileHashesUseRawBytesAndDoNotReturnPartialBatch(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "binary.dat")
	second := filepath.Join(root, "other.txt")
	if err := os.WriteFile(first, []byte{0, 0xff, 0x0a}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Files(SHA256, []string{first, second})
	if err != nil || !strings.HasSuffix(got, "  other.txt\r\n") || !strings.Contains(got, "  binary.dat\r\n") || strings.Count(got, "\r\n") != 2 {
		t.Fatalf("file report = %q, %v", got, err)
	}
	if got, err := Files(MD5, []string{first, filepath.Join(root, "missing")}); err == nil || got != "" {
		t.Fatalf("partial file report leaked: %q, %v", got, err)
	}
	if got, err := Files(MD5, nil); err != nil || got != "" {
		t.Fatalf("empty file report = %q, %v", got, err)
	}
}
