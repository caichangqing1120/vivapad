package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestASN1PaintingUpdatesFollowingLinesAfterQuoteEdits(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const source = "DESCRIPTION \"caf\u00e9\nBEGIN\"\nSYNTAX Integer32"
	doc.Editor.Insert(source)
	state.command("ASN.1")
	check := func(line string, color gfx.Color) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		snapshot := driver.Snapshot()
		for _, label := range snapshot.Texts {
			if strings.TrimSpace(label.Value) == line {
				for _, run := range snapshot.TextRuns(label) {
					if run.Color == color {
						return
					}
				}
			}
		}
		t.Errorf("editor did not paint %q in color %+v", line, color)
	}
	quoted, keyword := gfx.RGB(172, 77, 64), gfx.RGB(49, 92, 167)
	check(`BEGIN"`, quoted)
	check("SYNTAX Integer32", keyword)
	doc.Editor.SetSelection(text.Range{Start: 12, End: 13})
	doc.Editor.Insert(" ")
	check(`BEGIN"`, keyword)
	check("SYNTAX Integer32", quoted)
	if !doc.Editor.Undo() || doc.Editor.String() != source {
		t.Fatal("Undo did not restore the source")
	}
	check(`BEGIN"`, quoted)
	check("SYNTAX Integer32", keyword)
}
