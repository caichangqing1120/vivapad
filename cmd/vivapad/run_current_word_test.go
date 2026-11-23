package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/runcommand"
)

func TestRunCurrentWordPreservesContext(t *testing.T) {
	longWord := strings.Repeat("x", 1400)
	longSelection := strings.Repeat("\u754c", 1025)
	for _, test := range []struct {
		name, value string
		caret       int
		selection   text.Range
		want        string
	}{
		{name: "empty"},
		{name: "at the start", value: "\u03c0\u732b9_ other", want: "\u03c0\u732b9_"},
		{name: "separator after Unicode word", value: "\u03c0\u732b9_\u2026", caret: 4, want: "\u03c0\u732b9_"},
		{name: "at end of file", value: "left \u65e5\u672c9", caret: 8, want: "\u65e5\u672c9"},
		{name: "second separator", value: "left  right", caret: 5},
		{name: "selection includes separator", value: "x\u732b.\u03c0z", selection: text.Range{Start: 1, End: 4}, want: "\u732b.\u03c0"},
		{name: "selection limit counts runes", value: longSelection, selection: text.Range{Start: 0, End: 1025}, want: strings.Repeat("\u754c", 1024)},
		{name: "forward limit", value: longWord, caret: 4, want: longWord[:1024]},
		{name: "long word before caret", value: longWord, caret: 1200, want: longWord[:1201]},
	} {
		t.Run(test.name, func(t *testing.T) {
			editor := text.NewEditor(test.value)
			editor.MoveTo(test.caret, false)
			if test.selection.Start != test.selection.End {
				editor.SetSelection(test.selection)
			}
			doc := document.NewDraft("context.txt", editor)
			if got := runCurrentWord(doc); got != test.want {
				t.Fatalf("current word = %q (%d bytes), want %q (%d bytes)", got, len(got), test.want, len(test.want))
			}
		})
	}
}

func TestRunCurrentWordReusesCachedText(t *testing.T) {
	doc := largeRunContext()
	if got := runCurrentWord(doc); got != "\u732b9_" {
		t.Fatalf("current word = %q, want %q", got, "\u732b9_")
	}
	if allocs := testing.AllocsPerRun(3, func() { _ = runCurrentWord(doc) }); allocs != 0 {
		t.Fatalf("allocations for a cached document = %g, want zero", allocs)
	}
}

func largeRunContext() *document.Document {
	value := strings.Repeat("0123456789abcde\n", 1<<17) + "\u732b9_"
	return document.NewDraft("context.txt", text.NewEditor(value))
}

func BenchmarkRunVariablesLargeDocument(b *testing.B) {
	state := &model{}
	doc := largeRunContext()
	b.ReportAllocs()
	var variables runcommand.Variables
	for b.Loop() {
		variables = state.runVariables(doc)
	}
	if variables.Word != "\u732b9_" {
		b.Fatalf("current word = %q, want %q", variables.Word, "\u732b9_")
	}
}
