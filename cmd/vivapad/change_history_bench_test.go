package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkNavigateChangeSparseLargeDocument(b *testing.B) {
	path := filepath.Join(b.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("line\n", 100_000)), 0600); err != nil {
		b.Fatal(err)
	}
	state := newModel()
	if err := state.space.Open(path); err != nil {
		b.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.MoveTo(5, false)
	doc.Editor.Insert("X")
	for _, tc := range []struct {
		name     string
		start    int
		previous bool
	}{{"nearby", 0, false}, {"wrap", 5, true}} {
		b.Run(tc.name, func(b *testing.B) {
			doc.Editor.MoveTo(tc.start, false)
			state.navigateChange(tc.previous)
			if doc.Editor.Caret() != 5 || state.notice != "" {
				b.Fatalf("expected edited line, got caret %d and notice %q", doc.Editor.Caret(), state.notice)
			}
			b.ReportAllocs()
			for b.Loop() {
				doc.Editor.MoveTo(tc.start, false)
				state.navigateChange(tc.previous)
			}
		})
	}
}
