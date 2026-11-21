package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestANSIEditorRepaintsAfterChangingOneColorParameter(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const input = "\x1b[31;44mmessage\x1b[0m\nplain"
	doc.Editor.Insert(input)
	state.command("Escape Sequence (ANSI)")
	colored := func(want gfx.Color) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		snapshot := driver.Snapshot()
		for _, label := range snapshot.Texts {
			if strings.Contains(label.Value, "message") {
				for _, run := range snapshot.TextRuns(label) {
					if run.Color == want {
						return
					}
				}
			}
		}
		t.Fatalf("message did not paint in %v", want)
	}
	red, green := gfx.RGB(205, 0, 0), gfx.RGB(0, 205, 0)
	colored(red)
	doc.Editor.SetSelection(text.Range{Start: 3, End: 4})
	doc.Editor.Paste("2")
	colored(green)
	if !doc.Editor.Undo() || doc.Editor.String() != input {
		t.Fatal("one Undo did not restore the exact escape bytes")
	}
	colored(red)
}
