package datetime

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/text/language"
)

func TestCustomizedWindowsStyleTokensAndQuotedLiterals(t *testing.T) {
	now := time.Date(2026, time.November, 6, 15, 4, 5, 0, time.UTC)
	for _, test := range []struct{ pattern, want string }{
		{"yyyy-MM-dd HH:mm:ss", "2026-11-06 15:04:05"},
		{"ddd, MMM d yy 'at' h:m:s tt", "Fri, Nov 6 26 at 3:4:5 PM"},
		{"yyyy年M月d日 'it''s' HH:mm", "2026年11月6日 it's 15:04"},
		{"HH:mm 't' tt", "15:04 t PM"},
	} {
		got, err := Format(now, Customized, test.pattern, false, language.AmericanEnglish)
		if err != nil || got != test.want {
			t.Fatalf("pattern %q: got %q err=%v, want %q", test.pattern, got, err, test.want)
		}
	}
}

func TestCustomizedDateTimeRejectsInvalidPatternWithoutOutput(t *testing.T) {
	now := time.Date(2026, time.November, 6, 15, 4, 5, 0, time.UTC)
	for _, pattern := range []string{"", "yyyy 'unterminated", strings.Repeat("y", 129), "yyyy\nMM"} {
		if got, err := Format(now, Customized, pattern, false, language.English); err == nil || got != "" {
			t.Fatalf("invalid pattern %q returned %q, %v", pattern, got, err)
		}
	}
}
