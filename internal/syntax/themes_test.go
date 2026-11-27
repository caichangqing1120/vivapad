package syntax

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/components"
)

func TestSyntaxThemesRecolorCachedRunsWithoutChangingLexing(t *testing.T) {
	value := `return "word" // comment`
	var highlighter Highlighter
	classic := highlighter.Highlight(".go", value, nil)
	if len(classic) != 3 {
		t.Fatalf("keyword, string and comment runs: %+v", classic)
	}
	snapshot := slices.Clone(classic)
	modern := ModernDark.Recolor(slices.Clone(classic))
	monokai := Monokai.Recolor(slices.Clone(classic))
	for index, run := range classic {
		for _, themed := range []components.CodeEditorRun{modern[index], monokai[index]} {
			if themed.Start != run.Start || themed.End != run.End || themed.Color == run.Color {
				t.Fatalf("theme did not only recolor run %d: old=%+v new=%+v", index, run, themed)
			}
		}
		if modern[index].Color == monokai[index].Color {
			t.Fatalf("dark themes are not distinct for run %d", index)
		}
	}
	if !slices.Equal(highlighter.Highlight(".go", value, nil), snapshot) || !slices.Equal(ClassicLight.Recolor(classic), snapshot) {
		t.Fatal("dark theme polluted cached Classic Light colors")
	}
}
