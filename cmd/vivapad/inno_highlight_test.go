package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestInnoSetupPaintingFollowsSectionAndCommentEdits(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("[Code]\n(*\nprocedure Demo;\n*)\nBEGIN\n[Setup]\nAppName=Demo")
	state.command("Inno Setup")
	check := func(line string, want gfx.Color, colored bool) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		snapshot := driver.Snapshot()
		for _, label := range snapshot.Texts {
			if strings.TrimSpace(label.Value) != line {
				continue
			}
			found := false
			for _, run := range snapshot.TextRuns(label) {
				found = found || run.Color == want
			}
			if found != colored {
				t.Errorf("line %q has requested color = %v, want %v", line, found, colored)
			}
			return
		}
		t.Fatalf("editor did not draw %q", line)
	}
	comment, keyword := gfx.RGB(92, 125, 87), gfx.RGB(49, 92, 167)
	check("procedure Demo;", comment, true)
	check("BEGIN", keyword, true)
	check("AppName=Demo", keyword, true)
	doc.Editor.SetSelection(text.Range{Start: 1, End: 5})
	doc.Editor.Insert("Files")
	check("procedure Demo;", comment, false)
	check("BEGIN", keyword, false)
	if !doc.Editor.Undo() {
		t.Fatal("could not undo the section edit")
	}
	check("procedure Demo;", comment, true)
	doc.Editor.SetSelection(text.Range{Start: 7, End: 9})
	doc.Editor.Insert("  ")
	check("procedure Demo;", comment, false)
	check("procedure Demo;", keyword, true)
}
