package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestJSPEditorUpdatesColorAfterRemovingServerCommentOpener(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const opener = "<%--"
	const line = "<% return; %>"
	doc.Editor.Insert(opener + "\n" + line + "\n--%>")
	state.command("JSP")
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
	doc.Editor.Paste("    ")
	lineColor(keyword)
	if !doc.Editor.Undo() {
		t.Fatal("could not undo the edit")
	}
	lineColor(comment)
}
