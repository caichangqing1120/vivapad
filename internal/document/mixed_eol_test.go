package document

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/vivaui/viva/text"
)

func TestMixedEOLOpenSaveAndSaveCopyKeepOriginalBytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	original := []byte("a\r\nb\nc\rd\r\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil || !doc.MixedLineEndings() || doc.LineEnding() != CRLF || doc.Editor.String() != "a\nb\nc\nd\n" || doc.Dirty() {
		t.Fatalf("mixed open = %+v, %v", doc, err)
	}
	copyPath := filepath.Join(root, "copy.txt")
	if err := doc.SaveCopy(copyPath); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("mixed copy = %q, %v", data, err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, original) || doc.Dirty() {
		t.Fatalf("unchanged mixed Save = %q, %v dirty=%v", data, err, doc.Dirty())
	}
}

func TestMixedEOLEditsPreserveOldBreaksAndUseFirstForInsertedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("a\r\nb\nc\rd"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 2, End: 2}, "new\n")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\nnew\r\nb\nc\rd" {
		t.Fatalf("inserted line changed existing breaks = %q, %v", data, err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 6, End: 7}, "B")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\nnew\r\nB\nc\rd" {
		t.Fatalf("inline edit changed existing breaks = %q, %v", data, err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 7, End: 8}, "")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\nnew\r\nBc\rd" {
		t.Fatalf("deleted newline changed unrelated breaks = %q, %v", data, err)
	}
}

func TestExplicitConversionOfMixedEOLWritesUniformEvenWhenDefaultChosen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("a\r\nb\nc\rd"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.SetLineEnding(CRLF)
	if !doc.Dirty() {
		t.Fatal("choosing the default EOL did not mark mixed bytes dirty")
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\nb\r\nc\r\nd" || doc.MixedLineEndings() || doc.Dirty() {
		t.Fatalf("uniform CRLF save = %q, %v mixed=%v dirty=%v", data, err, doc.MixedLineEndings(), doc.Dirty())
	}
}

func TestMixedUTF16WithoutEditsRoundTripsIdentically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "utf16.txt")
	textValue := "猫\r\n犬\n鳥\r"
	units := utf16.Encode([]rune(textValue))
	raw := make([]byte, 2+2*len(units))
	raw[0], raw[1] = 0xff, 0xfe
	for index, unit := range units {
		binary.LittleEndian.PutUint16(raw[2+2*index:], unit)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil || doc.Encoding() != UTF16LE || !doc.MixedLineEndings() || doc.Editor.String() != "猫\n犬\n鳥\n" {
		t.Fatalf("mixed UTF-16 open = %+v, %v", doc, err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, raw) {
		t.Fatalf("mixed UTF-16 save changed bytes = %x, %v", data, err)
	}
}

func TestDeletingDefaultBreakDoesNotChangeRemainingUniformBreakOnSecondSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("a\r\nb\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: 2}, "")
	for attempt := 0; attempt < 2; attempt++ {
		if err := doc.Save(""); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != "b\n" {
			t.Fatalf("save %d changed surviving LF to %q: %v", attempt+1, data, err)
		}
	}
}

func TestSeparatedInsertAndDeleteKeepUntouchedMiddleLineEndings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("a\r\nb\nc\rd\r\ne"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 2, End: 2}, "new\n")
	value := doc.Editor.String()
	end := doc.Editor.RuneIndex(strings.LastIndex(value, "\n"))
	doc.Editor.ReplaceRange(text.Range{Start: end, End: end + 1}, "")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\nnew\r\nb\nc\rde" {
		t.Fatalf("separated edits moved untouched EOLs: %q, %v", data, err)
	}
}

func TestMixedEOLMappingProducesOneSeparatorPerEditedNewline(t *testing.T) {
	seeded := rand.New(rand.NewPCG(71, 9))
	insertions := []string{"", "X", "\n", "Y\nZ", "a\n\nb"}
	for iteration := 0; iteration < 300; iteration++ {
		before := "a\nb\nc\nd\ne"
		breaks := []string{"\r\n", "\n", "\r", "\r\n"}
		after := before
		for edit := 0; edit < 3; edit++ {
			start := seeded.IntN(len(after) + 1)
			end := start + seeded.IntN(len(after)-start+1)
			after = after[:start] + insertions[seeded.IntN(len(insertions))] + after[end:]
		}
		got := mapLineEndings(before, after, breaks, "\r\n")
		if len(got) != strings.Count(after, "\n") {
			t.Fatalf("iteration %d: before=%q after=%q separators=%q", iteration, before, after, got)
		}
		for _, ending := range got {
			if ending != "\n" && ending != "\r" && ending != "\r\n" {
				t.Fatalf("iteration %d: invalid physical separator %q", iteration, ending)
			}
		}
	}
}

func TestLargeMixedEOLKeepsAnchoredLinesAcrossDistantEdits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "many-lines.txt")
	var original, expected strings.Builder
	for index := 0; index < 300; index++ {
		line := fmt.Sprintf("line%03d", index)
		original.WriteString(line)
		if index < 299 {
			ending := []string{"\r\n", "\n", "\r"}[index%3]
			original.WriteString(ending)
		}
		expected.WriteString(line)
		if index == 0 {
			expected.WriteString("\r\nnew\r\n")
		} else if index < 298 {
			expected.WriteString([]string{"\r\n", "\n", "\r"}[index%3])
		}
	}
	if err := os.WriteFile(path, []byte(original.String()), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: len([]rune("line000\n")), End: len([]rune("line000\n"))}, "new\n")
	lastBreak := strings.LastIndex(doc.Editor.String(), "\n")
	doc.Editor.ReplaceRange(text.Range{Start: lastBreak, End: lastBreak + 1}, "")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != expected.String() {
		t.Fatalf("large anchored edit changed EOLs at byte %d: %v", firstDifference(data, []byte(expected.String())), err)
	}
}

func firstDifference(actual, expected []byte) int {
	for index := 0; index < len(actual) && index < len(expected); index++ {
		if actual[index] != expected[index] {
			return index
		}
	}
	return min(len(actual), len(expected))
}

func BenchmarkMapMixedEOLManyChangedLines(b *testing.B) {
	var before, after strings.Builder
	breaks := make([]string, 800)
	for index := range breaks {
		before.WriteString("old line\n")
		after.WriteString("new line\n")
		breaks[index] = "\r\n"
		if index%2 == 0 {
			breaks[index] = "\n"
		}
	}
	base, changed := before.String(), after.String()
	b.ReportAllocs()
	for b.Loop() {
		mapLineEndings(base, changed, breaks, "\r\n")
	}
}

func TestPhysicalUTF8BytesPreservesMixedLineEndingWidths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("a\r\nbé\nc\rd"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if total, selected := doc.PhysicalUTF8Bytes(1, 2); total != 10 || selected != 2 {
		t.Fatalf("CRLF selected: total=%d selected=%d", total, selected)
	}
	if total, selected := doc.PhysicalUTF8Bytes(3, 5); total != 10 || selected != 3 {
		t.Fatalf("UTF-8 plus LF selected: total=%d selected=%d", total, selected)
	}
	if doc.Dirty() || doc.Editor.String() != "a\nbé\nc\nd" {
		t.Fatal("byte counting changed the document")
	}
}
