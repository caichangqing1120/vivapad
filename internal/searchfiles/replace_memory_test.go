package searchfiles

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vivapad/internal/document"
)

func TestReplacePreviewReleasesFileContents(t *testing.T) {
	root := t.TempDir()
	const files = 16
	source := []byte(strings.Repeat("padding line\r\n", 16_384) + "needle\r\n")
	for i := range files {
		writeTestFile(t, filepath.Join(root, fmt.Sprintf("%02d.txt", i)), source)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	plan, err := PreviewReplace(t.Context(), root, "needle", "updated", document.SearchOptions{}, Options{})
	if err != nil || len(plan.Changes) != files || plan.Replacements != files {
		t.Fatalf("preview = %+v, %v", plan, err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(plan)
	retained := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("%d files, %d source bytes, %d additional live heap bytes", files, files*len(source), retained)
	// Allow ample runtime noise but reject retaining even one copy of every
	// source file while the confirmation is open.
	if retained > int64(files*len(source)/2) {
		t.Fatalf("preview retained %d bytes of live heap", retained)
	}
}
