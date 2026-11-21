package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestBaanCEditorRecolorsAfterEditingUsageBlock(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const opener = "functionusage"
	const line = "SELECT abort.io()"
	doc.Editor.Insert(opener + "\n" + line + "\nendfunctionusage")
	state.command("BaanC")
	lineColor := func(want gfx.Color) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		snapshot := driver.Snapshot()
		for _, label := range snapshot.Texts {
			if label.Value == line {
				for _, run := range snapshot.TextRuns(label) {
					if run.Color == want {
						return
					}
				}
			}
		}
		t.Fatalf("%q did not draw in %v", line, want)
	}
	comment, keyword := gfx.RGB(92, 125, 87), gfx.RGB(49, 92, 167)
	lineColor(comment)
	doc.Editor.SetSelection(text.Range{Start: 0, End: len(opener)})
	doc.Editor.Paste("| heading")
	lineColor(keyword)
	lineColor(gfx.RGB(113, 76, 144))
	if !doc.Editor.Undo() {
		t.Fatal("could not undo the opener edit")
	}
	lineColor(comment)
}
