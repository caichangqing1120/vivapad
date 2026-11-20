package websearch

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const Placeholder = "$(CURRENT_WORD)"

type Config struct {
	Engine   string `json:"engine,omitempty"`
	Template string `json:"template,omitempty"`
}

type Engine struct{ ID, Label, template string }

var engines = [...]Engine{
	{"duckduckgo", "DuckDuckGo", "https://duckduckgo.com/?q=" + Placeholder},
	{"google", "Google", "https://www.google.com/search?q=" + Placeholder},
	{"yahoo", "Yahoo", "https://search.yahoo.com/search?q=" + Placeholder},
	{"stackoverflow", "Stack Overflow", "https://stackoverflow.com/search?q=" + Placeholder},
	{"custom", "Custom", ""},
}

func Engines() []Engine { return append([]Engine(nil), engines[:]...) }

func (c Config) EngineID() string {
	if c.Engine == "" {
		return engines[0].ID
	}
	return c.Engine
}

func (c Config) Validate() error {
	_, _, _, err := c.target()
	return err
}

func (c Config) URL(query string) (string, error) {
	if len(query) > 1024*utf8.UTFMax || strings.ContainsAny(query, "\x00\r\n") || !utf8.ValidString(query) || utf8.RuneCountInString(query) > 1024 || strings.TrimSpace(query) == "" {
		return "", errors.New("select a single nonempty query of at most 1024 characters")
	}
	parsed, values, key, err := c.target()
	if err != nil {
		return "", err
	}
	values.Set(key, query)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func (c Config) target() (*url.URL, url.Values, string, error) {
	var template string
	for _, engine := range engines {
		if engine.ID == c.EngineID() {
			template = engine.template
			if engine.ID == "custom" {
				template = c.Template
			} else if c.Template != "" {
				return nil, nil, "", errors.New("only a custom engine accepts a URL template")
			}
			break
		}
	}
	if len(template) > 2048 || strings.Count(template, Placeholder) != 1 || strings.IndexFunc(template, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || r == '\\' || r == '#'
	}) >= 0 {
		return nil, nil, "", errors.New("provide an HTTPS URL with one $(CURRENT_WORD) query value")
	}
	parsed, err := url.Parse(template)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" ||
		strings.Contains(parsed.Path, Placeholder) || strings.Contains(parsed.Host, Placeholder) {
		return nil, nil, "", errors.New("search URL must use HTTPS without credentials or a fragment")
	}
	values, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return nil, nil, "", errors.New("invalid URL query parameters")
	}
	key := ""
	for candidate, params := range values {
		if strings.Contains(candidate, Placeholder) {
			return nil, nil, "", errors.New("the placeholder must be a query value")
		}
		for _, param := range params {
			if strings.Contains(param, Placeholder) {
				if key != "" || candidate == "" || param != Placeholder || len(params) != 1 {
					return nil, nil, "", errors.New("the placeholder must be the only value of its query parameter")
				}
				key = candidate
			}
		}
	}
	if key == "" {
		return nil, nil, "", errors.New("the placeholder must be a query value")
	}
	return parsed, values, key, nil
}
