package main

import (
	"testing"

	"golang.org/x/text/language"
)

func TestSortLanguageUsesWindowsUserDefaultNotPreferredUI(t *testing.T) {
	for _, test := range []struct {
		platform  string
		preferred []string
		want      language.Tag
	}{
		{"windows", []string{"sv-SE", "en-US"}, language.Und},
		{"windows", nil, language.Und},
		{"darwin", []string{"sv-SE", "en-US"}, language.Make("sv-SE")},
		{"linux", nil, language.Und},
	} {
		calls := 0
		if got := sortLanguageForPlatform(test.platform, func() []string { calls++; return test.preferred }); got != test.want {
			t.Fatalf("%s sort locale = %s want %s", test.platform, got, test.want)
		}
		if test.platform == "windows" && calls != 0 {
			t.Fatal("Windows NLS default consulted the preferred UI languages")
		}
	}
}
