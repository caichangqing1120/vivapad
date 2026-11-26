package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestResourceEditorTracksBlockCommentsAcrossEdits(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const opener = "/* comment"
	const body = "DIALOGEX"
	const closer = "MENU */ STRINGTABLE"
	const quote = `CAPTION "escaped \" /*"`
	const lineComment = "// /*"
	doc.Editor.Insert(strings.Join([]string{opener, body, closer, quote, lineComment, "BEGIN"}, "\n"))
	state.command("Resource file")
	checkColor := func(line, token string, want gfx.Color) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		snapshot := driver.Snapshot()
		start := strings.Index(line, token)
		for _, label := range snapshot.Texts {
			if label.Value != line {
				continue
			}
			offset, covered := 0, 0
			for _, run := range snapshot.TextRuns(label) {
				end := offset + int(run.Bytes)
				if run.Color == want {
					covered += max(0, min(end, start+len(token))-max(offset, start))
				}
				offset = end
			}
			if covered == len(token) {
				return
			}
		}
		t.Errorf("%q in %q did not draw in %v", token, line, want)
	}
	comment, keyword := gfx.RGB(92, 125, 87), gfx.RGB(49, 92, 167)
	checkColor(body, body, comment)
	checkColor(closer, "MENU", comment)
	checkColor(closer, "STRINGTABLE", keyword)
	checkColor("BEGIN", "BEGIN", keyword)
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: 2}, "  ")
	checkColor(body, body, keyword)
	if !doc.Editor.Undo() {
		t.Fatal("could not undo opener edit")
	}
	checkColor(body, body, comment)
}
