package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestKixPaintingUpdatesAfterBlockOpenerEdits(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const source = "/* comment\nRETURN 2\n*/\nDIM $name"
	doc.Editor.Insert(source)
	state.command("KIXtart")
	check := func(color gfx.Color) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		snapshot := driver.Snapshot()
		for _, label := range snapshot.Texts {
			if strings.TrimSpace(label.Value) == "RETURN 2" {
				for _, run := range snapshot.TextRuns(label) {
					if run.Color == color {
						return
					}
				}
			}
		}
		t.Errorf("editor did not paint RETURN 2 in color %+v", color)
	}
	comment, keyword := gfx.RGB(92, 125, 87), gfx.RGB(49, 92, 167)
	check(comment)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	doc.Editor.Insert(" ")
	check(keyword)
	if !doc.Editor.Undo() || doc.Editor.String() != source {
		t.Fatal("Undo did not restore the source")
	}
	check(comment)
}
