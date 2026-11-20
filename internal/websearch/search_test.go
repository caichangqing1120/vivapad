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
