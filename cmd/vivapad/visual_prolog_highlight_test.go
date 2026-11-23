package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestVisualPrologPaintPreservesVerbatimStringsAndRecolorsAfterEdit(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("@<open\n/* literal text>\nclass demo")
	state.command("Visual Prolog")
	check := func(line string, color gfx.Color) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		snapshot := driver.Snapshot()
		for _, label := range snapshot.Texts {
			if label.Value == line {
				for _, run := range snapshot.TextRuns(label) {
					if run.Color == color {
						return
					}
				}
			}
		}
		t.Fatalf("line %q did not draw in expected color %v", line, color)
	}
	stringColor, commentColor, keywordColor := gfx.RGB(172, 77, 64), gfx.RGB(92, 125, 87), gfx.RGB(113, 76, 144)
	check("/* literal text>", stringColor)
	check("class demo", keywordColor)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	doc.Editor.Paste("  ")
	check("/* literal text>", commentColor)
	check("class demo", commentColor)
	if !doc.Editor.Undo() {
		t.Fatal("could not undo opener edit")
	}
	check("/* literal text>", stringColor)
	check("class demo", keywordColor)
}
