package main

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"vivapad/internal/runcommand"
)

func TestRunCurrentLineExpandsOnlyCompleteUTF8Characters(t *testing.T) {
	for _, tc := range []struct {
		name, line, want string
	}{
		{"short line", "  \u732b's \U0001f642 ", "  \u732b's \U0001f642 "},
		{"complete at limit", strings.Repeat("a", 4093) + "\u732brest", strings.Repeat("a", 4093) + "\u732b"},
		{"split CJK", strings.Repeat("a", 4094) + "\u732brest", strings.Repeat("a", 4094)},
		{"split emoji", strings.Repeat("a", 4095) + "\U0001f642rest", strings.Repeat("a", 4095)},
		{"empty line", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := newModelWithFiles(nil)
			doc := state.space.Current()
			doc.Editor.Insert("previous\n" + tc.line + "\nnext")
			// A caret past the limit must still export the start of the same line.
			doc.Editor.MoveTo(utf8.RuneCountInString("previous\n"+tc.line), false)

			var launched runcommand.Prepared
			calls := 0
			state.runLauncher = func(prepared runcommand.Prepared) error {
				calls++
				launched = prepared
				return nil
			}
			if err := state.launchRunTemplate(`echo "line=$(CURRENT_LINESTR)" $(CURRENT_LINE)`); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || launched.Executable != "echo" || len(launched.Args) != 2 {
				t.Fatalf("expected one prepared call with two arguments, got %d calls and %d arguments", calls, len(launched.Args))
			}
			if !utf8.ValidString(launched.Args[0]) {
				t.Fatalf("expanded line is invalid UTF-8 (%d bytes)", len(launched.Args[0])-len("line="))
			}
			if !slices.Equal(launched.Args, []string{"line=" + tc.want, "2"}) {
				t.Fatalf("expanded line has %d bytes, want %d; line number = %q, want 2",
					len(launched.Args[0])-len("line="), len(tc.want), launched.Args[1])
			}
		})
	}
}
