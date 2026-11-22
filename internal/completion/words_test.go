package completion

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
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

func TestWordsKeepsCharacterOffsetsAcrossDifferentFoldWidths(t *testing.T) {
	value := "\U0001f9ea Keep kEpt Kestrel \u212aettle \u212aept \u212aept-tail"
	caret := utf8.RuneCountInString(value) - len("pt-tail")
	got := Words(value, caret, true)
	if want := (Result{Start: caret - 2, End: caret, Candidates: []string{"Keep", "kEpt", "Kestrel", "\u212aettle"}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("folded results = %+v, want %+v", got, want)
	}
}

func TestWordsAppliesByteLimitToUnicodePrefixAndCandidates(t *testing.T) {
	prefix := strings.Repeat("\u4e2d", 84) + "ab"
	short := prefix + "c"
	long := prefix + "cd"
	value := short + " " + long + " " + prefix
	caret := utf8.RuneCountInString(value)
	if got := Words(value, caret, false); got.Start != caret-86 || got.End != caret || !reflect.DeepEqual(got.Candidates, []string{short}) {
		t.Fatalf("255-byte boundary: start=%d end=%d candidates=%d", got.Start, got.End, len(got.Candidates))
	}
	if got := Words(value+"cd", caret+2, false); !reflect.DeepEqual(got, Result{}) {
		t.Fatalf("256-byte prefix was accepted: %+v", got)
	}
}

func TestWordsAllocationsStayBoundedWithUnmatchedTokens(t *testing.T) {
	value, caret := largeWordCompletionFixture()
	var got Result
	allocations := testing.AllocsPerRun(3, func() { got = Words(value, caret, false) })
	if !reflect.DeepEqual(got.Candidates, []string{"\u00c9clair", "\u00c9cole"}) {
		t.Fatalf("large document candidates: %v", got.Candidates)
	}
	if allocations > 12 {
		t.Fatalf("lookup allocated %.0f times with just two matches", allocations)
	}
}

func BenchmarkWordsLargeDocument(b *testing.B) {
	value, caret := largeWordCompletionFixture()
	b.SetBytes(int64(len(value)))
	b.ReportAllocs()
	for b.Loop() {
		Words(value, caret, false)
	}
}

func largeWordCompletionFixture() (string, int) {
	value := strings.Repeat("logger timestamp failed trace status \n", 48000) + "\u00c9clair \u00c9cole \u00c9c"
	return value, utf8.RuneCountInString(value)
}
