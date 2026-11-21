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
	if err := os.WriteFile(path, []byte(strings.Repeat("line\r\nother\n", 50_000)+"needle aaa"), 0600); err != nil {
		b.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	docs := []*document.Document{doc}
	for _, tc := range []struct {
		name, query, replacement string
		regex                    bool
	}{
		{name: "unmatched", query: "unmatched", replacement: "unmatched"},
		{name: "needle", query: "needle", replacement: "needle"},
		{name: "net-equal", query: `(a)a|(a)`, replacement: `$1$2$2`, regex: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				plan, err := PreviewOpenReplace(docs, tc.query, tc.replacement, document.SearchOptions{MatchCase: true, Regex: tc.regex})
				if err != nil || len(plan.Changes) != 0 {
					b.Fatalf("unexpected replacement plan: %+v, %v", plan, err)
				}
			}
		})
	}
}
