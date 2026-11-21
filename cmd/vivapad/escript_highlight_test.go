package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestESCRIPTEditorRecolorsAfterChangingCommentDelimiter(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("/* opener\nFunction demo()\nEndfunction")
	state.command("ESCRIPT")
	lineColor := func(want gfx.Color) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		snapshot := driver.Snapshot()
		for _, label := range snapshot.Texts {
			if label.Value == "Function demo()" {
				for _, run := range snapshot.TextRuns(label) {
					if run.Color == want {
						return
					}
				}
			}
		}
		t.Fatalf("function header did not draw in %v", want)
	}
	comment, keyword := gfx.RGB(92, 125, 87), gfx.RGB(113, 76, 144)
	lineColor(comment)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	doc.Editor.Paste("  ")
	lineColor(keyword)
	if !doc.Editor.Undo() {
		t.Fatal("could not undo the comment edit")
	}
	lineColor(comment)
}
