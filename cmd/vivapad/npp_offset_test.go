package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

// abc, U+4E2D, U+1F600, CRLF, x, LF, z, CR: 15 disk bytes, 16 UTF-8 bytes,
// and 10 runes after the editor normalizes line endings.
const nppOffsetLine = "abc\xd6\xd0\x94\x39\xfc\x36\r\nx\nz\r"
const nppOffsetUTF8Line = "abc\u4e2d\U0001f600\r\nx\nz\r"

func nppOffsetFixture(t testing.TB, lines int) *document.Document {
	t.Helper()
	path := filepath.Join(t.TempDir(), "offsets.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat(nppOffsetLine, lines)), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.OpenWithEncoding(path, document.GB18030)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestNPPPositionRestoresGB18030MixedEOLWithoutPerRuneAllocations(t *testing.T) {
	const lines = 1 << 12
	doc := nppOffsetFixture(t, lines)
	base := int64((lines - 1) * len(nppOffsetUTF8Line))
	for _, tc := range []struct {
		add  int64
		want int
	}{
		{4, 3},  // Halfway through U+4E2D.
		{8, 4},  // Inside the four-byte U+1F600.
		{11, 5}, // The second byte of CRLF.
		{12, 6}, // Just after CRLF.
		{15, 9}, // The last CR.
		{16, 10},
		{40, 10}, // Past EOF.
	} {
		if got, want := nppSelectionAt(doc, base+tc.add, base+tc.add).Start, (lines-1)*10+tc.want; got != want {
			t.Fatalf("GB18030 offset %d = %d, want %d", base+tc.add, got, want)
		}
	}
	if got, want := nppSelectionAt(doc, base+12, base+4), (text.Range{Start: (lines-1)*10 + 6, End: (lines-1)*10 + 3}); got != want {
		t.Fatalf("reversed GB18030 selection = %+v, want %+v", got, want)
	}
	allocations := testing.AllocsPerRun(2, func() {
		nppSelectionAt(doc, base+4, base+12)
	})
	if allocations > 256 {
		t.Errorf("one legacy selection used %.0f allocations for repeated characters; want at most 256", allocations)
	}
}

func BenchmarkNPPLegacySelectionOffsets(b *testing.B) {
	const lines = 1 << 17
	doc := nppOffsetFixture(b, lines)
	base := int64((lines - 1) * len(nppOffsetUTF8Line))
	b.ReportAllocs()
	b.SetBytes(int64(lines * len(nppOffsetUTF8Line)))
	for b.Loop() {
		if got, want := nppSelectionAt(doc, base+4, base+12), (text.Range{Start: (lines-1)*10 + 3, End: (lines-1)*10 + 6}); got != want {
			b.Fatalf("wrong selection: %+v, want %+v", got, want)
		}
	}
}
