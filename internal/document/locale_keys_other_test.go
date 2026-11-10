//go:build !windows

package document

import (
	"testing"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func TestOwnedLocaleKeysPreserveNonWindowsCollation(t *testing.T) {
	values := []string{"a", "A", "ä", "z", "2", "10", "item2", "item10", " ", "\x00", "\u200b", "\U00010000", "猫", "."}
	for _, locale := range []language.Tag{language.English, language.Swedish, language.Turkish} {
		keys := newLocaleKeys(locale)
		comparator := collate.New(locale, collate.IgnoreCase, collate.Numeric)
		for _, left := range values {
			for _, right := range values {
				first, firstErr := keys.key(left)
				second, secondErr := keys.key(right)
				comparison := compareLocaleKeys(first, second)
				want := comparator.CompareString(left, right)
				if firstErr != nil || secondErr != nil || (comparison < 0) != (want < 0) || (comparison > 0) != (want > 0) {
					t.Fatalf("%s materialized comparison %q/%q differs: %d want %d (%v %v)", locale, left, right, comparison, want, firstErr, secondErr)
				}
			}
		}
		if key, err := keys.key(""); err != nil || key != nil {
			t.Fatal("empty source key must remain distinct from a nonempty ignorable key")
		}
	}
}
