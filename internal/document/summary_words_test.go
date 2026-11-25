package document

import (
	"regexp"
	"testing"
)

func TestSummaryWordsMatchPinnedNotepadPlusPlus(t *testing.T) {
	// Notepad++ v8.9.8.1, Notepad_plus::wordCount, transcribed as a Go regexp.
	sourcePattern := regexp.MustCompile(`[^ \t\\.,;:!?()+\r\n\-\*/=\]\[{}&~"'\x60|@$%<>\^]+`)
	check := func(t *testing.T, value string) {
		t.Helper()
		doc := New("summary.txt")
		doc.Editor.Insert(value)
		want := len(sourcePattern.FindAllStringIndex(value, -1))
		if got := doc.SummaryCounts(0, 1).Words; got != want {
			t.Errorf("summary words for %q = %d, pinned Notepad++ pattern = %d", value, got, want)
		}
	}
	t.Run("every printable ASCII character", func(t *testing.T) {
		for r := rune(' '); r <= '~'; r++ {
			check(t, "a"+string(r)+"b")
		}
	})
	for _, tc := range []struct{ name, value string }{
		{"combining marks and symbols", "cafe\u0301 \u0301 \U0001f642 # _ a\u00a0b"},
		{"mixed line endings and tab", "alpha\r\nbeta\ngamma\rdelta\tepsilon"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check(t, tc.value)
		})
	}
}
