package searchfiles

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vivapad/internal/document"
)

func BenchmarkFindLargeMixedEOLFile(b *testing.B) {
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte(strings.Repeat("line\r\nother\nthird\r", 34_000)+"needle"), 0600); err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		query string
		count int
	}{{"unmatched", 0}, {"needle", 1}} {
		b.Run(tc.query, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				result, err := Find(context.Background(), root, tc.query, document.SearchOptions{MatchCase: true}, Options{})
				if err != nil || len(result.Matches) != tc.count || result.FilesSearched != 1 {
					b.Fatalf("unexpected search result: matches=%d files=%d error=%v", len(result.Matches), result.FilesSearched, err)
				}
			}
		})
	}
}
