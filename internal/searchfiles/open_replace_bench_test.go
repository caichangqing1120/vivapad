package searchfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vivapad/internal/document"
)

func BenchmarkPreviewOpenReplaceUnchangedDocument(b *testing.B) {
	path := filepath.Join(b.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("line\r\nother\n", 50_000)+"needle"), 0600); err != nil {
		b.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	docs := []*document.Document{doc}
	for _, query := range []string{"unmatched", "needle"} {
		b.Run(query, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				plan, err := PreviewOpenReplace(docs, query, query, document.SearchOptions{MatchCase: true})
				if err != nil || len(plan.Changes) != 0 {
					b.Fatalf("unexpected replacement plan: %+v, %v", plan, err)
				}
			}
		})
	}
}
