package syntax

import "testing"

func TestGoAndJSONHighlightUseRealLexers(t *testing.T) {
	var highlighter Highlighter
	goRuns := highlighter.Highlight(".go", "func main() {", nil)
	if len(goRuns) == 0 || goRuns[0].Start != 0 || goRuns[0].End != 4 {
		t.Fatalf("Go keyword runs = %+v", goRuns)
	}
	jsonRuns := highlighter.Highlight(".json", `{"name": 42}`, nil)
	if len(jsonRuns) < 2 {
		t.Fatalf("JSON value runs = %+v", jsonRuns)
	}
	if plain := highlighter.Highlight(".txt", "func main() {", nil); len(plain) != 0 {
		t.Fatalf("plain text was highlighted: %+v", plain)
	}
}

func TestHighlightCacheDoesNotReturnStaleLine(t *testing.T) {
	var highlighter Highlighter
	first := highlighter.Highlight(".go", "func one() {}", nil)
	second := highlighter.Highlight(".go", "func two() {}", nil)
	if len(first) == 0 || len(second) == 0 {
		t.Fatalf("missing runs: %+v / %+v", first, second)
	}
	if repeated := highlighter.Highlight(".go", "func one() {}", nil); len(repeated) != len(first) {
		t.Fatalf("cached runs changed: %+v / %+v", first, repeated)
	}
}
