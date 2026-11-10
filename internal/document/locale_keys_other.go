//go:build !windows

package document

import "golang.org/x/text/language"

func newLocaleKeys(locale language.Tag) localeKeys {
	return newUnicodeLocaleKeys(locale)
}
