package searchfiles

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestFindFiltersSkipsAndReportsUnicodePosition(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "one.go"), []byte("猫 cat\ncat_ cat\n"))
	writeTestFile(t, filepath.Join(root, "nested", "two.go"), []byte("CAT\n"))
	writeTestFile(t, filepath.Join(root, "other.txt"), []byte("cat"))
	writeTestFile(t, filepath.Join(root, "bad.go"), []byte{'c', 0, 'a', 't'})
	writeTestFile(t, filepath.Join(root, "large.go"), make([]byte, 101))
	result, err := Find(context.Background(), root, "cat", document.SearchOptions{WholeWord: true}, Options{
		Pattern: "*.go", Recursive: false, MaxFileBytes: 100, MaxMatches: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesSearched != 1 || result.FilesSkipped != 2 || len(result.Matches) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if first := result.Matches[0]; first.Line != 1 || first.Column != 3 || first.Start != len("猫 ") || first.Preview != "猫 cat" {
		t.Fatalf("first match = %+v", first)
	}
	if second := result.Matches[1]; second.Line != 2 || second.Column != 6 {
		t.Fatalf("second match = %+v", second)
	}
	result, err = Find(context.Background(), root, "cat", document.SearchOptions{WholeWord: true}, Options{
		Pattern: "*.go", Recursive: true, MaxFileBytes: 100, MaxMatches: 2,
	})
	if err != nil || len(result.Matches) != 2 || !result.Truncated {
		t.Fatalf("capped recursive result = %+v, %v", result, err)
	}
}

func TestFindReadsUTF16AndRejectsInvalidInputs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "unicode.txt"), []byte{0xff, 0xfe, 0x2d, 0x4e})
	result, err := Find(context.Background(), root, "中", document.SearchOptions{}, Options{Pattern: "*.txt", Recursive: true})
	if err != nil || len(result.Matches) != 1 || result.Matches[0].Column != 1 {
		t.Fatalf("UTF-16 search = %+v, %v", result, err)
	}
	if _, err := Find(context.Background(), root, "(", document.SearchOptions{Regex: true}, Options{}); err == nil {
		t.Fatal("invalid regex accepted")
	}
	if _, err := Find(context.Background(), root, "中", document.SearchOptions{}, Options{Pattern: "["}); err == nil {
		t.Fatal("invalid glob accepted")
	}
	contextValue, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Find(contextValue, root, "中", document.SearchOptions{}, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled search = %v", err)
	}
}

func TestFindBindsMixedEOLResultToDecodedTextAndOriginalBytes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		bytes      string
		read, want document.Encoding
	}{
		{"Unicode BOM", "\xef\xbb\xbfa\r\nb\rc\ncaf\xc3\xa9 cat", document.UTF8, document.UTF8BOM},
		{"explicit Windows-1252", "a\r\nb\rc\ncaf\xe9 cat", document.Windows1252, document.Windows1252},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "source.txt")
			writeTestFile(t, file, []byte(tc.bytes))
			result, err := Find(t.Context(), root, "cat", document.SearchOptions{}, Options{Encoding: tc.read})
			if err != nil || len(result.Matches) != 1 || result.FilesSkipped != 0 {
				t.Fatalf("search result = %+v, %v", result, err)
			}
			match := result.Matches[0]
			const prefix = "a\nb\nc\ncaf\u00e9 "
			if !match.MatchesRange(prefix+"cat") || match.MatchesRange("z"+(prefix + "cat")[1:]) ||
				match.Encoding != tc.want || match.Line != 4 || match.Column != 6 ||
				match.Start != len(prefix) || match.End != len(prefix)+3 ||
				match.DiskDigest != fmt.Sprintf("%x", sha256.Sum256([]byte(tc.bytes))) {
				t.Fatalf("match lost its source coordinates, encoding or disk identity: %+v", match)
			}
		})
	}
}

func TestLongLinePreviewIncludesTheMatch(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "long.txt"), []byte(strings.Repeat("中", 200)+"needle"+strings.Repeat("文", 200)))
	result, err := Find(context.Background(), root, "needle", document.SearchOptions{}, Options{Pattern: "*.txt"})
	if err != nil || len(result.Matches) != 1 || !strings.Contains(result.Matches[0].Preview, "needle") ||
		len([]rune(result.Matches[0].Preview)) > 166 {
		t.Fatalf("long-line preview = %+v, %v", result, err)
	}
}

func BenchmarkLongLinePreview(b *testing.B) {
	line := "needle" + strings.Repeat("文", 1<<20)
	b.ResetTimer()
	for range b.N {
		if preview := previewLine(line, 0, len("needle")); !strings.HasPrefix(preview, "needle") {
			b.Fatalf("long-line preview = %q", preview)
		}
	}
}

func TestLongLinePreviewPreservesUnicodeWindowAndEllipses(t *testing.T) {
	line := strings.Repeat("文", 120) + "needle" + strings.Repeat("字", 120)
	start := len(strings.Repeat("文", 120))
	want := "..." + strings.Repeat("文", 60) + "needle" + strings.Repeat("字", 94) + "..."
	if got := previewLine(line, start, start+len("needle")); got != want {
		t.Fatalf("unicode preview = %q, want %q", got, want)
	}
	short := "  猫 cat  "
	if got := previewLine(short, len("  猫 "), len("  猫 cat")); got != "猫 cat" {
		t.Fatalf("short-line preview = %q", got)
	}
}

func TestManyResultsOnLongUnicodeLineKeepColumns(t *testing.T) {
	const prefixRunes = 500000
	doc := document.New("new 1")
	doc.Editor.Insert(strings.Repeat("文", prefixRunes) + strings.Repeat("cat ", 2000))
	result, err := FindDocument(doc, "cat", document.SearchOptions{}, 2000)
	if err != nil || result.Truncated || len(result.Matches) != 2000 {
		t.Fatalf("long-line results: count=%d truncated=%v err=%v", len(result.Matches), result.Truncated, err)
	}
	if last := result.Matches[1999]; last.Line != 1 || last.Column != prefixRunes+7997 || last.Text != "cat" {
		t.Fatalf("last long-line match: line=%d column=%d text=%q", last.Line, last.Column, last.Text)
	}
}

func BenchmarkManyResultsOnLongUnicodeLine(b *testing.B) {
	doc := document.New("new 1")
	doc.Editor.Insert(strings.Repeat("文", 500000) + strings.Repeat("cat ", 2000))
	b.ResetTimer()
	for range b.N {
		result, err := FindDocument(doc, "cat", document.SearchOptions{}, 2000)
		if err != nil || len(result.Matches) != 2000 {
			b.Fatalf("long-line search: matches=%d err=%v", len(result.Matches), err)
		}
	}
}

func TestExactMatchLimitIsNotTruncated(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "one.txt"), []byte("one"))
	result, err := Find(context.Background(), root, "one", document.SearchOptions{}, Options{MaxMatches: 1})
	if err != nil || len(result.Matches) != 1 || result.Truncated {
		t.Fatalf("exact limit = %+v, %v", result, err)
	}
	if _, err := PreviewReplace(context.Background(), root, "one", "two", document.SearchOptions{}, Options{MaxMatches: 1}); err != nil {
		t.Fatalf("exact-limit replacement preview failed: %v", err)
	}
}

func TestMatchLimitAcrossDirectoryFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.txt"), []byte("one"))
	writeTestFile(t, filepath.Join(root, "b.txt"), []byte("zero"))
	options := Options{MaxMatches: 1}
	result, err := Find(context.Background(), root, "one", document.SearchOptions{}, options)
	if err != nil || len(result.Matches) != 1 || result.Truncated || result.FilesSearched != 2 {
		t.Fatalf("exact cross-file limit: matches=%d truncated=%v searched=%d err=%v", len(result.Matches), result.Truncated, result.FilesSearched, err)
	}
	writeTestFile(t, filepath.Join(root, "c.txt"), []byte("one"))
	result, err = Find(context.Background(), root, "one", document.SearchOptions{}, options)
	if err != nil || len(result.Matches) != 1 || !result.Truncated || result.FilesSearched != 3 {
		t.Fatalf("truncated cross-file limit: matches=%d truncated=%v searched=%d err=%v", len(result.Matches), result.Truncated, result.FilesSearched, err)
	}
	if _, err := PreviewReplace(context.Background(), root, "one", "two", document.SearchOptions{}, options); err == nil {
		t.Fatal("a truncated replacement preview was accepted")
	}
}

func TestFindLegacyFileWithExplicitEncoding(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "chinese.txt"), []byte{0xd6, 0xd0, 0xce, 0xc4, ' ', 0xd6, 0xd0})
	plain, err := Find(context.Background(), root, "中", document.SearchOptions{}, Options{Pattern: "*.txt"})
	if err != nil || len(plain.Matches) != 0 || plain.FilesSkipped != 1 {
		t.Fatalf("default strict scan = %+v, %v", plain, err)
	}
	result, err := Find(context.Background(), root, "中", document.SearchOptions{}, Options{Pattern: "*.txt", Encoding: document.GB18030})
	if err != nil || len(result.Matches) != 2 || result.FilesSkipped != 0 || result.Matches[1].Column != 4 || result.Matches[0].Encoding != document.GB18030 {
		t.Fatalf("GB18030 scan = %+v, %v", result, err)
	}
	if _, err := Find(context.Background(), root, "中", document.SearchOptions{}, Options{Encoding: document.Encoding(99)}); err == nil {
		t.Fatal("unknown explicit directory encoding was accepted")
	}
}

func TestFindRegisteredCharacterSetFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "japanese.txt"), []byte{0x93, 0xfa, 0x96, 0x7b})
	result, err := Find(context.Background(), root, "日本", document.SearchOptions{}, Options{Pattern: "*.txt", Encoding: document.ShiftJIS})
	if err != nil || len(result.Matches) != 1 || result.Matches[0].Encoding != document.ShiftJIS ||
		result.Matches[0].Start != 0 || result.Matches[0].End != len("日本") || !result.Matches[0].MatchesRange("日本") {
		t.Fatalf("Shift-JIS directory result = %+v, %v", result, err)
	}
}

func TestFindDocumentIncludesUnsavedTextAndUnicodePositions(t *testing.T) {
	doc := document.New("new 1")
	doc.Editor.Insert("猫 cat\ncat_ cat\nCAT")
	result, err := FindDocument(doc, "cat", document.SearchOptions{WholeWord: true}, 2)
	if err != nil || result.FilesSearched != 1 || len(result.Matches) != 2 || !result.Truncated {
		t.Fatalf("in-memory results = %+v, %v", result, err)
	}
	if first := result.Matches[0]; first.Path != "" || first.Line != 1 || first.Column != 3 || first.Preview != "猫 cat" || first.Start != len("猫 ") {
		t.Fatalf("first in-memory match = %+v", first)
	}
	if second := result.Matches[1]; second.Line != 2 || second.Column != 6 || second.Text != "cat" {
		t.Fatalf("second in-memory match = %+v", second)
	}
	if _, err := FindDocument(doc, "(", document.SearchOptions{Regex: true}, 2); err == nil {
		t.Fatal("invalid in-memory regex accepted")
	}
	if _, err := FindDocument(doc, "", document.SearchOptions{}, 2); err == nil {
		t.Fatal("empty in-memory search accepted")
	}
}

func TestFindDocumentUsesCommittedSnapshotDuringPreedit(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		options     document.SearchOptions
	}{
		{"literal", "cat", document.SearchOptions{}},
		{"bounded regex", "c.t", document.SearchOptions{Regex: true}},
		{"zero width regex fallback", `cat|(?m:^$)`, document.SearchOptions{Regex: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := document.New("draft")
			const committed = "猫 cat\ncat"
			doc.Editor.Insert(committed)
			doc.Editor.SetSelection(text.Range{Start: 2, End: 5})
			doc.Editor.SetMarkedText("dog\ncat cat", text.Range{})
			visible := doc.Editor.String()
			if visible == committed || doc.Editor.CommittedString() != committed {
				t.Fatal("precondition: marked text did not replace the first match")
			}
			result, err := FindDocument(doc, tc.query, tc.options, 2)
			if err != nil || result.Truncated || len(result.Matches) != 2 {
				t.Fatalf("results from the committed snapshot = %+v, %v", result, err)
			}
			first, second := result.Matches[0], result.Matches[1]
			if first.Source != committed || first.Start != len("猫 ") || first.Line != 1 || first.Column != 3 || first.Text != "cat" ||
				second.Source != committed || second.Start != len("猫 cat\n") || second.Line != 2 || second.Text != "cat" {
				t.Fatalf("matches used provisional text or offsets: %+v", result.Matches)
			}
			if !doc.Editor.CompositionActive() || doc.Editor.String() != visible {
				t.Fatal("search altered the live preedit")
			}
		})
	}
}

func TestLiteralSearchLimitStopsBeforeEnumeratingWholeDocument(t *testing.T) {
	const occurrences = 200000
	doc := document.New("new 1")
	doc.Editor.Insert(strings.Repeat("a", occurrences))
	var result Result
	var searchErr error
	allocations := testing.AllocsPerRun(1, func() {
		result, searchErr = FindDocument(doc, "a", document.SearchOptions{}, 2)
	})
	if searchErr != nil || len(result.Matches) != 2 || !result.Truncated || result.Matches[1].Start != 1 {
		t.Fatalf("capped in-memory search: matches=%d truncated=%v err=%v", len(result.Matches), result.Truncated, searchErr)
	}
	if allocations > 500 {
		t.Fatalf("capped in-memory search allocated %.0f times for %d occurrences", allocations, occurrences)
	}

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "many.txt"), []byte(strings.Repeat("a", occurrences)))
	allocations = testing.AllocsPerRun(1, func() {
		result, searchErr = Find(context.Background(), root, "a", document.SearchOptions{}, Options{MaxMatches: 2})
	})
	if searchErr != nil || len(result.Matches) != 2 || !result.Truncated || result.FilesSearched != 1 {
		t.Fatalf("capped directory search: matches=%d truncated=%v searched=%d err=%v", len(result.Matches), result.Truncated, result.FilesSearched, searchErr)
	}
	if allocations > 500 {
		t.Fatalf("capped directory search allocated %.0f times for %d occurrences", allocations, occurrences)
	}
}

func TestPositiveWidthRegexLimitStopsBeforeEnumeratingWholeDocument(t *testing.T) {
	doc := document.New("new 1")
	doc.Editor.Insert(strings.Repeat("a", 200000))
	var result Result
	var searchErr error
	allocations := testing.AllocsPerRun(1, func() {
		result, searchErr = FindDocument(doc, "a+?", document.SearchOptions{Regex: true}, 2)
	})
	if searchErr != nil || len(result.Matches) != 2 || !result.Truncated || result.Matches[1].Start != 1 {
		t.Fatalf("capped regex search: matches=%d truncated=%v err=%v", len(result.Matches), result.Truncated, searchErr)
	}
	if allocations > 500 {
		t.Fatalf("capped positive-width regex allocated %.0f times", allocations)
	}
}

func TestWholeWordRegexLimitDoesNotRetainRejectedMatches(t *testing.T) {
	doc := document.New("new 1")
	doc.Editor.Insert(strings.Repeat("cat_", 300000) + " cat")
	options := document.SearchOptions{Regex: true, WholeWord: true}
	previousGC := debug.SetGCPercent(25)
	defer debug.SetGCPercent(previousGC)
	peakHeap := func(search func()) uint64 {
		runtime.GC()
		var before, current runtime.MemStats
		runtime.ReadMemStats(&before)
		peak := before.HeapAlloc
		done := make(chan struct{})
		go func() {
			defer close(done)
			search()
		}()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				runtime.ReadMemStats(&current)
				return max(peak, current.HeapAlloc) - before.HeapAlloc
			case <-ticker.C:
				runtime.ReadMemStats(&current)
				peak = max(peak, current.HeapAlloc)
			}
		}
	}
	fullBytes := peakHeap(func() {
		matches, err := doc.MatchesWith("cat", options)
		if err != nil || len(matches) != 1 {
			t.Fatalf("reference full search: matches=%d err=%v", len(matches), err)
		}
	})
	limitedBytes := peakHeap(func() {
		result, err := FindDocument(doc, "cat", options, 1)
		if err != nil || result.Truncated || len(result.Matches) != 1 || result.Matches[0].Start != 1200001 {
			t.Fatalf("limited regex whole word: matches=%d truncated=%v err=%v", len(result.Matches), result.Truncated, err)
		}
	})
	t.Logf("whole-word regex peak heap: limited=%dB full=%dB", limitedBytes, fullBytes)
	if limitedBytes*2 >= fullBytes {
		t.Fatalf("limited regex search retained rejected hits: limited=%dB full=%dB", limitedBytes, fullBytes)
	}
}

func TestLiteralWholeWordLimitCountsOnlyAcceptedMatches(t *testing.T) {
	doc := document.New("new 1")
	doc.Editor.Insert(strings.Repeat("cat_", 4000) + " cat cats cat")
	result, err := FindDocument(doc, "cat", document.SearchOptions{WholeWord: true}, 2)
	if err != nil || result.Truncated || len(result.Matches) != 2 {
		t.Fatalf("whole-word limit: matches=%d truncated=%v err=%v", len(result.Matches), result.Truncated, err)
	}
	if first, last := result.Matches[0], result.Matches[1]; first.Start != 16001 || last.Start != 16010 || first.Column != 16002 {
		t.Fatalf("whole-word matches: first=(%d,%d) last=(%d,%d)", first.Start, first.Column, last.Start, last.Column)
	}
	result, err = FindDocument(doc, "cat", document.SearchOptions{WholeWord: true}, 1)
	if err != nil || !result.Truncated || len(result.Matches) != 1 || result.Matches[0].Start != 16001 {
		t.Fatalf("whole-word truncated: matches=%d truncated=%v err=%v", len(result.Matches), result.Truncated, err)
	}
}

func TestDirectorySearchCanCancelWithinOneFileWithoutAcceptedMatches(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "many.txt"), []byte(strings.Repeat("cat_\n", 200000)))
	for _, test := range []struct {
		name, query string
		regex       bool
		wholeWord   bool
	}{
		{"literal", "cat", false, true},
		{"regex", "cat", true, true},
		{"anchored regex", `(?m)^cat`, true, true},
		{"zero-width regex", `\b`, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := &cancelAfterChecks{limit: 20}
			result, err := Find(ctx, root, test.query, document.SearchOptions{WholeWord: test.wholeWord, Regex: test.regex}, Options{MaxMatches: 2})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("single-file search was not canceled: checks=%d searched=%d err=%v", ctx.calls, result.FilesSearched, err)
			}
		})
	}
}
