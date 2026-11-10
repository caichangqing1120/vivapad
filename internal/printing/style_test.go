package printing

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/syntax"
)

func TestColorizeUsesEditorSyntaxRunsOnOriginalByteSpans(t *testing.T) {
	job := Job{Language: ".go", Text: "func main() {\n\treturn 1\n}", Settings: Settings{LineNumbers: true, ColorMode: ColorOnWhite}}
	device := testDevice()
	device.PaperWidth = 200
	pages, err := Paginate(job, device, testTextMetrics(), PageVariables{})
	if err != nil || len(pages) == 0 {
		t.Fatalf("pages = %+v, %v", pages, err)
	}
	var highlighter syntax.Highlighter
	colored, err := Colorize(job, pages, highlighter.Highlight)
	if err != nil || len(colored) != len(pages) || len(pages[0].Styles) != 0 {
		t.Fatalf("style output changed input pages: %+v, %v", colored, err)
	}
	keyword := ColorRun{StartByte: 0, EndByte: 4, Color: ui.RGB(49, 92, 167)}
	if len(colored[0].Styles) != len(colored[0].Rows) || len(colored[0].Styles[0]) == 0 || colored[0].Styles[0][0] != keyword {
		t.Fatalf("first keyword style = %+v, want %+v", colored[0].Styles, keyword)
	}
	for index, page := range colored {
		for rowIndex, row := range page.Rows {
			for _, run := range page.Styles[rowIndex] {
				if run.StartByte < row.StartByte || run.EndByte > row.EndByte || run.StartByte >= run.EndByte {
					t.Fatalf("page %d row %d style escapes source span: %+v / %+v", index, rowIndex, run, row)
				}
			}
		}
	}
}

func TestColorizeClipsRunsAcrossSoftWrapAndRejectsInvalidLexerOutput(t *testing.T) {
	job := Job{Text: "abcdef", Settings: Settings{ColorMode: ColorOnWhite}}
	device := testDevice()
	device.PaperWidth = 55
	pages, err := Paginate(job, device, testTextMetrics(), PageVariables{})
	if err != nil || len(pages[0].Rows) != 3 {
		t.Fatalf("wrapped source = %+v, %v", pages, err)
	}
	color := ui.RGB(172, 77, 64)
	calls := 0
	all := func(string, string, []components.CodeEditorRun) []components.CodeEditorRun {
		calls++
		return []components.CodeEditorRun{{Start: 1, End: 5, Color: color}}
	}
	colored, err := Colorize(job, pages, all)
	if err != nil || calls != 1 || len(colored[0].Styles[0]) != 1 || colored[0].Styles[0][0] != (ColorRun{StartByte: 1, EndByte: 2, Color: color}) ||
		len(colored[0].Styles[1]) != 1 || colored[0].Styles[1][0] != (ColorRun{StartByte: 2, EndByte: 4, Color: color}) ||
		len(colored[0].Styles[2]) != 1 || colored[0].Styles[2][0] != (ColorRun{StartByte: 4, EndByte: 5, Color: color}) {
		t.Fatalf("wrapped style spans = %+v, %v", colored, err)
	}
	invalid := func(string, string, []components.CodeEditorRun) []components.CodeEditorRun {
		return []components.CodeEditorRun{{Start: -1, End: 2}}
	}
	if result, err := Colorize(job, pages, invalid); err == nil || result != nil || len(pages[0].Styles) != 0 {
		t.Fatalf("invalid lexer output was accepted or mutated pages: %+v, %v", result, err)
	}
}

func TestColorizeKeepsOriginalByteCoordinatesAcrossExpandedTab(t *testing.T) {
	job := Job{Text: "a\tb", Settings: Settings{ColorMode: ColorOnWhite}}
	pages, err := Paginate(job, testDevice(), testTextMetrics(), PageVariables{})
	if err != nil || len(pages) != 1 || pages[0].Rows[0].Text != "a   b" {
		t.Fatalf("expanded tab row = %+v, %v", pages, err)
	}
	color := ui.RGB(172, 77, 64)
	colored, err := Colorize(job, pages, func(string, string, []components.CodeEditorRun) []components.CodeEditorRun {
		return []components.CodeEditorRun{{Start: 1, End: 2, Color: color}}
	})
	if err != nil || len(colored[0].Styles[0]) != 1 || colored[0].Styles[0][0] != (ColorRun{StartByte: 1, EndByte: 2, Color: color}) {
		t.Fatalf("tab color uses display rather than source bytes: %+v, %v", colored, err)
	}
}
