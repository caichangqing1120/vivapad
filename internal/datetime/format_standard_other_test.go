//go:build !windows

package datetime

import (
	"testing"
	"time"

	"golang.org/x/text/language"
)

func TestShortAndLongDateTimeFollowLocaleAndOrder(t *testing.T) {
	now := time.Date(2026, time.November, 6, 15, 4, 5, 0, time.UTC)
	for _, test := range []struct {
		kind    Kind
		locale  language.Tag
		reverse bool
		want    string
	}{
		{Short, language.AmericanEnglish, false, "3:04 PM 11/6/2026"},
		{Short, language.AmericanEnglish, true, "11/6/2026 3:04 PM"},
		{Long, language.AmericanEnglish, false, "3:04 PM Friday, November 6, 2026"},
		{Short, language.SimplifiedChinese, false, "15:04 2026/11/6"},
		{Long, language.SimplifiedChinese, true, "2026年11月6日 星期五 15:04"},
	} {
		got, err := Format(now, test.kind, "", test.reverse, test.locale)
		if err != nil || got != test.want {
			t.Fatalf("%v %v reverse=%v: got %q err=%v, want %q", test.kind, test.locale, test.reverse, got, err, test.want)
		}
	}
}
