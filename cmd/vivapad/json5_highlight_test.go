package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestJSON5PaintingTracksCommentContextAfterEditingEarlierLines(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("/*\ninside: 42\n*/\noutside: 42")
	state.command("JSON5")
	pump := func() {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
	colored := func(line string, color gfx.Color) bool {
		t.Helper()
		snapshot := driver.Snapshot()
		for _, label := range snapshot.Texts {
			if strings.TrimSpace(label.Value) != line {
				continue
			}
			for _, run := range snapshot.TextRuns(label) {
				if run.Color == color {
					return true
				}
			}
			return false
		}
		t.Fatalf("editor did not draw %q", line)
		return false
	}
	comment, property := gfx.RGB(92, 125, 87), gfx.RGB(49, 92, 167)
	pump()
	if !colored("inside: 42", comment) || !colored("outside: 42", property) {
		t.Error("comment context did not reach the next line or leaked past its closer")
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 2})
	doc.Editor.Insert("  ")
	pump()
	if colored("inside: 42", comment) || !colored("inside: 42", property) {
		t.Error("editing the opener left stale highlighting on the next line")
	}
	if !doc.Editor.Undo() {
		t.Fatal("could not undo the delimiter edit")
	}
	pump()
	if !colored("inside: 42", comment) {
		t.Error("undo did not restore the comment context")
	}
}
