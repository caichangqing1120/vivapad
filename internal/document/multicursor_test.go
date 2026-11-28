package document

import (
	"slices"
	"testing"
)

func TestMultiCursorPasteParts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		count int
		want  []string
	}{
		{"matching mixed separators", "α\r\nβ\rγ\nδ", 4, []string{"α", "β", "γ", "δ"}},
		{"mismatch normalized as one block", "α\r\nβ\rγ\nδ", 3, []string{"α\nβ\nγ\nδ"}},
		{"empty lines count", "\r\nβ\r", 3, []string{"", "β", ""}},
		{"single caret", "α\r\nβ", 1, []string{"α\nβ"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MultiCursorPasteParts(tc.value, tc.count); !slices.Equal(got, tc.want) {
				t.Fatalf("paste parts = %q, want %q", got, tc.want)
			}
		})
	}
}
