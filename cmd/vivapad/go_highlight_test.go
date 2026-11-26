package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestGoEditorTracksMultilineTokensAcrossEdits(t *testing.T) {
	keyword := gfx.RGB(49, 92, 167)
	for _, tc := range []struct {
		name, opener, closer string
		delimiter            string
		color                gfx.Color
	}{
		{"block comment", "/* opening", "inside */; var tail = 1", "/*", gfx.RGB(92, 125, 87)},
		{"raw string", "var message = `opening", "inside`; var tail = 1", "`", gfx.RGB(172, 77, 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			const body = "func textInside() {}"
			const outside = "var outside = true"
			doc.Editor.Insert(strings.Join([]string{
				tc.opener, body, tc.closer,
				"// /* `", `var quoted = "escaped \" /* ` + "`" + `"`, "var runeLiteral = '`'", outside,
			}, "\n"))
			state.command("Go")
			check := func(line, token string, want gfx.Color) {
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
			check(tc.opener, "opening", tc.color)
			check(body, "func", tc.color)
			check(tc.closer, "inside", tc.color)
			check(tc.closer, "var", keyword)
			check(outside, "var", keyword)
			start := strings.Index(tc.opener, tc.delimiter)
			doc.Editor.ReplaceRange(text.Range{Start: start, End: start + len(tc.delimiter)}, strings.Repeat(" ", len(tc.delimiter)))
			check(body, "func", keyword)
			if !doc.Editor.Undo() {
				t.Fatal("could not undo opener edit")
			}
			check(body, "func", tc.color)
		})
	}
}
