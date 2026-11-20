package searchfiles

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vivapad/internal/document"
)

func TestDirectoryFindDoesNotRetainMatchedFileContents(t *testing.T) {
	root := t.TempDir()
	const files = 16
	source := []byte("needle\r\n" + strings.Repeat("padding line\r\n", 16_384))
	for i := range files {
		writeTestFile(t, filepath.Join(root, fmt.Sprintf("%02d.txt", i)), source)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	result, err := Find(t.Context(), root, `(?s)^needle.*$`, document.SearchOptions{Regex: true}, Options{})
	if err != nil || result.Truncated || len(result.Matches) != files {
		t.Fatalf("whole-file search returned %d matches (truncated=%v): %v", len(result.Matches), result.Truncated, err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(result)
	retained := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("%d files, %d source bytes, %d additional live heap bytes", files, files*len(source), retained)
	if retained > int64(files*len(source)/2) {
		t.Fatalf("Finder retained %d bytes of live heap", retained)
	}
}
