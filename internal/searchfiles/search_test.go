package searchfiles

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestLongLinePreviewIncludesTheMatch(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "long.txt"), []byte(strings.Repeat("中", 200)+"needle"+strings.Repeat("文", 200)))
	result, err := Find(context.Background(), root, "needle", document.SearchOptions{}, Options{Pattern: "*.txt"})
	if err != nil || len(result.Matches) != 1 || !strings.Contains(result.Matches[0].Preview, "needle") ||
		len([]rune(result.Matches[0].Preview)) > 166 {
		t.Fatalf("long-line preview = %+v, %v", result, err)
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
	if err != nil || len(result.Matches) != 1 || result.Matches[0].Encoding != document.ShiftJIS || result.Matches[0].Text != "日本" {
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
