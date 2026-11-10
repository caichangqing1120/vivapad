package document

import (
	"bytes"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

type unicodeLocaleKeys struct {
	locale   language.Tag
	collator *collate.Collator
	buffer   *collate.Buffer
}

func newUnicodeLocaleKeys(locale language.Tag) localeKeys {
	return &unicodeLocaleKeys{locale: locale}
}

func (keys *unicodeLocaleKeys) key(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	if keys.collator == nil {
		keys.collator = collate.New(keys.locale, collate.IgnoreCase, collate.Numeric)
		keys.buffer = &collate.Buffer{}
	}
	keys.buffer.Reset()
	result := bytes.Clone(keys.collator.KeyFromString(keys.buffer, value))
	if result == nil {
		result = []byte{}
	}
	return result, nil
}
