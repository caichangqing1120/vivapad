package completion

import (
	"reflect"
	"strings"
	"testing"
)

func TestWordsExcludesCurrentTokenAndSortsCaseSensitiveCandidates(t *testing.T) {
	result := Words("alpha alphabet alp", 18, false)
	if result.Start != 15 || result.End != 18 || !reflect.DeepEqual(result.Candidates, []string{"alpha", "alphabet"}) {
		t.Fatalf("word candidates = %+v", result)
	}
	inside := Words("alpha alphabet", 2, false)
	if inside.Start != 0 || inside.End != 2 || !reflect.DeepEqual(inside.Candidates, []string{"alphabet"}) {
		t.Fatalf("word under caret was not excluded: %+v", inside)
	}
	if got := Words("alpha ", 6, false); len(got.Candidates) != 0 {
		t.Fatalf("word completion began at whitespace: %+v", got)
	}
}

func TestWordsHandlesUnicodeAndLanguageCaseMode(t *testing.T) {
	unicode := Words("日本語 日本", 6, false)
	if unicode.Start != 4 || unicode.End != 6 || !reflect.DeepEqual(unicode.Candidates, []string{"日本語"}) {
		t.Fatalf("Unicode candidates = %+v", unicode)
	}
	value := "Alpha alphabet ALP"
	if got := Words(value, 18, false); len(got.Candidates) != 0 {
		t.Fatalf("plain text ignored case: %+v", got)
	}
	caseFold := Words(value, 18, true)
	if !reflect.DeepEqual(caseFold.Candidates, []string{"Alpha", "alphabet"}) {
		t.Fatalf("language candidates = %+v", caseFold)
	}
	if got := Words(strings.Repeat("a", 256)+" alphabet", 256, true); len(got.Candidates) != 0 {
		t.Fatalf("oversized prefix was accepted: %+v", got)
	}
}
