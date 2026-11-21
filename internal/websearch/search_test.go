package websearch

import (
	"net/url"
	"testing"
)

func TestSearchURLKeepsQueryInOneValue(t *testing.T) {
	const query = "猫 & q=split#% 東京"
	for _, config := range []Config{{}, {Engine: "google"}, {Engine: "yahoo"}, {Engine: "stackoverflow"},
		{Engine: "custom", Template: "https://example.test/find?scope=editor&term=$(CURRENT_WORD)"}} {
		target, err := config.URL(query)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(target)
		if err != nil || parsed.Scheme != "https" || parsed.Fragment != "" {
			t.Fatalf("target %q: %v", target, err)
		}
		key := "q"
		if config.Engine == "custom" {
			key = "term"
			if parsed.Host != "example.test" || parsed.Query().Get("scope") != "editor" || len(parsed.Query()) != 2 {
				t.Fatalf("template changed: %q", target)
			}
		}
		if got := parsed.Query()[key]; len(got) != 1 || got[0] != query {
			t.Fatalf("query values = %q", got)
		}
	}
}

func TestSearchURLRejectsLineSeparators(t *testing.T) {
	for _, separator := range []rune{'\r', '\n', '\v', '\f', '\u0085', '\u2028', '\u2029'} {
		if target, err := (Config{}).URL("alpha" + string(separator) + "beta"); err == nil || target != "" {
			t.Errorf("U+%04X unexpectedly built a search URL: %q, %v", separator, target, err)
		}
	}
	const horizontal = "alpha\u00a0beta\tgamma"
	target, err := (Config{}).URL(horizontal)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Query().Get("q") != horizontal {
		t.Fatalf("horizontal whitespace changed: %q, %v", target, err)
	}
}

func TestSearchTemplateRejectsAmbiguousOrUnsafeTarget(t *testing.T) {
	for _, template := range []string{
		`http://example.test/?q=$(CURRENT_WORD)`, `https://user@example.test/?q=$(CURRENT_WORD)`,
		`https://example.test/$(CURRENT_WORD)?q=x`, `https://example.test/?$(CURRENT_WORD)=a`,
		`https://example.test/?q=x$(CURRENT_WORD)`, `https://example.test/?q=$(CURRENT_WORD)&q=x`,
		`https://example.test/?q=$(CURRENT_WORD)&other=$(CURRENT_WORD)`,
		`https://example.test/?q=$(CURRENT_WORD)#fragment`, `https://example.test/?q=$(CURRENT_WORD)&a=%XX`,
	} {
		if got, err := (Config{Engine: "custom", Template: template}).URL("search"); err == nil || got != "" {
			t.Errorf("accepted %q: %q, %v", template, got, err)
		}
	}
}
