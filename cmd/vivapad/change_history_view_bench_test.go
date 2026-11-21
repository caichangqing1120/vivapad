package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vivapad/internal/document"
)

func BenchmarkChangeMarginSparseLargeDocument(b *testing.B) {
	path := filepath.Join(b.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("line\n", 100_000)), 0600); err != nil {
		b.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	doc.Editor.MoveTo(5, false)
	doc.Editor.Insert("X")
	state := &model{}
	if bars := state.changeMarginLines(doc); len(bars) != 1 || bars[0].Line != 1 {
		b.Fatalf("expected one edited line, got %+v", bars)
	}
	b.ReportAllocs()
	for b.Loop() {
		state.forgetChangeView(doc)
		state.changeMarginLines(doc)
	}
}
