package searchfiles

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vivapad/internal/document"
)

func TestReplacePreviewDoesNotWriteAndApplyPreservesFormat(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.txt")
	second := filepath.Join(root, "second.txt")
	writeTestFile(t, first, []byte("cat CAT\r\n"))
	writeTestFile(t, second, []byte("cat"))
	plan, err := PreviewReplace(context.Background(), root, "cat", "dog", document.SearchOptions{}, Options{Pattern: "*.txt", Recursive: true})
	if err != nil || len(plan.Changes) != 2 || plan.Replacements != 3 {
		t.Fatalf("preview = %+v, %v", plan, err)
	}
	if before, _ := os.ReadFile(first); string(before) != "cat CAT\r\n" {
		t.Fatalf("preview wrote disk: %q", before)
	}
	result, err := plan.Apply(context.Background())
	if err != nil || result.FilesChanged != 2 || result.Replacements != 3 {
		t.Fatalf("apply = %+v, %v", result, err)
	}
	if changed, _ := os.ReadFile(first); string(changed) != "dog dog\r\n" {
		t.Fatalf("CRLF changed to %q", changed)
	}
	if changed, _ := os.ReadFile(second); string(changed) != "dog" {
		t.Fatalf("second file = %q", changed)
	}
}

func TestDirectoryPreviewOmitsByteIdenticalReplacement(t *testing.T) {
	for _, test := range []struct {
		name, query, replacement string
		regex                    bool
	}{
		{"literal", "cat", "cat", false},
		{"regex capture", "(cat)", "$1", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "identity.txt")
			const original = "\xef\xbb\xbfcat\r\ncat\ncat\r"
			writeTestFile(t, path, []byte(original))
			plan, err := PreviewReplace(context.Background(), root, test.query, test.replacement,
				document.SearchOptions{MatchCase: true, Regex: test.regex}, Options{Pattern: "*.txt"})
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Changes) != 0 || plan.Replacements != 0 || plan.FilesSearched != 1 {
				t.Fatalf("byte-identical preview reported %d files and %d replacements: %+v", len(plan.Changes), plan.Replacements, plan)
			}
		})
	}
}

func TestDirectoryReplaceKeepsUnchangedFileIdentityInMixedBatch(t *testing.T) {
	root := t.TempDir()
	unchanged, changed := filepath.Join(root, "a.txt"), filepath.Join(root, "z.txt")
	const source = "\xef\xbb\xbfcat\r\ncat\nlast\r"
	writeTestFile(t, unchanged, []byte(source))
	writeTestFile(t, changed, []byte("cat CAT\r\nlast"))
	before, err := os.Stat(unchanged)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewReplace(context.Background(), root, "cat", "cat", document.SearchOptions{}, Options{Pattern: "*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 || filepath.Base(plan.Changes[0].Path) != "z.txt" || plan.Replacements != 2 {
		t.Fatalf("mixed batch preview = %+v", plan)
	}
	result, err := plan.Apply(context.Background())
	if err != nil || result.FilesChanged != 1 || result.Replacements != 2 {
		t.Fatalf("apply = %+v, %v", result, err)
	}
	after, err := os.Stat(unchanged)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("applying the mixed batch touched the unchanged file")
	}
	if disk, err := os.ReadFile(unchanged); err != nil || string(disk) != source {
		t.Fatalf("unchanged disk bytes = %x, %v", disk, err)
	}
	if disk, err := os.ReadFile(changed); err != nil || string(disk) != "cat cat\r\nlast" {
		t.Fatalf("changed disk bytes = %q, %v", disk, err)
	}
}

func TestReplaceInFilesPreservesMixedLineEndings(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	writeTestFile(t, path, []byte("cat\r\ncat\ncat\r"))
	plan, err := PreviewReplace(context.Background(), root, "cat", "dog", document.SearchOptions{}, Options{Pattern: "*.txt"})
	if err != nil || plan.Replacements != 3 {
		t.Fatalf("mixed preview = %+v, %v", plan, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "cat\r\ncat\ncat\r" {
		t.Fatalf("preview changed mixed source = %q, %v", data, err)
	}
	if _, err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "dog\r\ndog\ndog\r" {
		t.Fatalf("replacement normalized mixed EOL = %q, %v", data, err)
	}
}

func TestReplacePreflightRejectsChangedFileBeforeAnyWrite(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a.txt")
	second := filepath.Join(root, "z.txt")
	writeTestFile(t, first, []byte("one"))
	writeTestFile(t, second, []byte("one"))
	plan, err := PreviewReplace(context.Background(), root, "one", "two", document.SearchOptions{}, Options{Pattern: "*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("external one"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := plan.Apply(context.Background())
	if err == nil || result.FilesChanged != 0 {
		t.Fatalf("stale preflight = %+v, %v", result, err)
	}
	if content, _ := os.ReadFile(first); string(content) != "one" {
		t.Fatalf("first file was changed before preflight completed: %q", content)
	}
}

func TestReplaceRegexCapturesAndTruncation(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "data.txt"), []byte("item-12 item-34"))
	plan, err := PreviewReplace(context.Background(), root, `(item)-(\d+)`, `${2}:$1`, document.SearchOptions{Regex: true}, Options{MaxMatches: 10})
	if err != nil || plan.Replacements != 2 {
		t.Fatalf("regex preview = %+v, %v", plan, err)
	}
	if _, err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if content, _ := os.ReadFile(filepath.Join(root, "data.txt")); string(content) != "12:item 34:item" {
		t.Fatalf("capture output = %q", content)
	}
	if _, err := PreviewReplace(context.Background(), root, "item", "x", document.SearchOptions{}, Options{MaxMatches: 1}); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("truncated preview was accepted: %v", err)
	}
}

type cancelAfterChecks struct {
	calls int
	limit int
}

func (contextValue *cancelAfterChecks) Deadline() (time.Time, bool) { return time.Time{}, false }
func (contextValue *cancelAfterChecks) Done() <-chan struct{}       { return nil }
func (contextValue *cancelAfterChecks) Value(any) any               { return nil }
func (contextValue *cancelAfterChecks) Err() error {
	contextValue.calls++
	if contextValue.calls >= contextValue.limit {
		return context.Canceled
	}
	return nil
}

func TestReplaceReportsPartialBatchOnCancellation(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "a.txt"), filepath.Join(root, "z.txt")
	writeTestFile(t, first, []byte("one"))
	writeTestFile(t, second, []byte("one"))
	plan, err := PreviewReplace(context.Background(), root, "one", "two", document.SearchOptions{}, Options{Pattern: "*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := plan.Apply(&cancelAfterChecks{limit: 4})
	if err == nil || result.FilesChanged != 1 || result.Replacements != 1 || !strings.Contains(err.Error(), "after 1 files") {
		t.Fatalf("partial batch = %+v, %v", result, err)
	}
	if firstData, _ := os.ReadFile(first); string(firstData) != "two" {
		t.Fatalf("first file = %q", firstData)
	}
	if secondData, _ := os.ReadFile(second); string(secondData) != "one" {
		t.Fatalf("second file = %q", secondData)
	}
}

func TestReplaceLegacyFilePreservesEncodingAndCRLF(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "chinese.txt")
	writeTestFile(t, path, []byte{0xd6, 0xd0, '\r', '\n', 0xd6, 0xd0})
	plan, err := PreviewReplace(context.Background(), root, "中", "文", document.SearchOptions{}, Options{Pattern: "*.txt", Encoding: document.GB18030})
	if err != nil || plan.Replacements != 2 || len(plan.Changes) != 1 || plan.Changes[0].Encoding != document.GB18030 {
		t.Fatalf("legacy preview = %+v, %v", plan, err)
	}
	if data, _ := os.ReadFile(path); string(data) != string([]byte{0xd6, 0xd0, '\r', '\n', 0xd6, 0xd0}) {
		t.Fatalf("preview changed disk: %x", data)
	}
	if _, err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != string([]byte{0xce, 0xc4, '\r', '\n', 0xce, 0xc4}) {
		t.Fatalf("replacement lost legacy format: %x", data)
	}
}

func TestUnmappableLegacyBatchFailsBeforePreviewOrWrites(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a.txt")
	second := filepath.Join(root, "b.txt")
	writeTestFile(t, first, []byte("cat"))
	writeTestFile(t, second, []byte{'c', 'a', 'f', 0xe9, ' ', 'c', 'a', 't'})
	if _, err := PreviewReplace(context.Background(), root, "cat", "中文", document.SearchOptions{}, Options{Pattern: "*.txt", Encoding: document.Windows1252}); err == nil {
		t.Fatal("unmappable batch was offered for confirmation")
	}
	if data, _ := os.ReadFile(first); string(data) != "cat" {
		t.Fatalf("first file changed: %x", data)
	}
	if data, _ := os.ReadFile(second); string(data) != string([]byte{'c', 'a', 'f', 0xe9, ' ', 'c', 'a', 't'}) {
		t.Fatalf("second file changed: %x", data)
	}
}
