//go:build windows

package document

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/text/language"
)

func TestNativeWindowsLocaleKeysUseNLSFlagsAndExplicitCounts(t *testing.T) {
	name, err := resolveNLSLocale(language.English)
	if err != nil {
		t.Fatal(err)
	}
	if resolved := windows.UTF16ToString(name); !strings.HasPrefix(strings.ToLower(resolved), "en-") {
		t.Fatalf("neutral language was not resolved: %q", resolved)
	} else {
		t.Logf("native resolved locale: %s", resolved)
	}
	keys := &nlsLocaleKeys{locale: name, mapString: func(locale []uint16, flags uint32, source []uint16, destination []byte) (int, error) {
		runtime.GC()
		return mapNLSString(locale, flags, source, destination)
	}}
	key := func(value string) []byte {
		t.Helper()
		result, err := keys.key(value)
		if err != nil || result == nil {
			t.Fatalf("native key %q: %x %v", value, result, err)
		}
		return result
	}
	if compareLocaleKeys(key("2"), key("10")) >= 0 {
		t.Fatal("native NLS did not use digit-as-number ordering")
	}
	if !bytes.Equal(key("a"), key("A")) {
		t.Fatal("native NLS did not ignore linguistic case")
	}
	if bytes.Equal(key("a\x00b"), key("a")) {
		t.Fatal("native NLS source was truncated at embedded NUL")
	}
	_ = key("\U0001f600猫")
}

func TestNativeWindowsLocaleDefaultAndCulturalOrdering(t *testing.T) {
	name, err := resolveNLSLocale(language.Und)
	if err != nil || name != nil {
		t.Fatalf("Windows user default is not a null locale: %v %v", name, err)
	}
	keys := newLocaleKeys(language.Und).(*windowsLocaleKeys)
	if _, err := keys.key("2"); err != nil || keys.native.locale != nil {
		t.Fatalf("native user-default key lookup failed: %v", err)
	}
	swedish := newLocaleKeys(language.Swedish)
	first, firstErr := swedish.key("z")
	second, secondErr := swedish.key("ä")
	if firstErr != nil || secondErr != nil || compareLocaleKeys(first, second) >= 0 {
		t.Fatalf("native Swedish ordering: %x %x %v %v", first, second, firstErr, secondErr)
	}
}

func TestNativeWindowsLocaleDocumentAndRectangleHistory(t *testing.T) {
	doc := New("native-nls.txt")
	before := "item10\nItem2\nitem1\nITEM2\n"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	if accepted, err := doc.SortLinesLocaleWithError(language.Und, true); err != nil || !accepted || doc.Editor.String() != "item1\nItem2\nITEM2\nitem10\n" {
		t.Fatalf("native document sort: %q %v %v", doc.Editor.String(), accepted, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("native document sort did not retain one Undo unit")
	}
	doc = New("native-rectangle.txt")
	doc.Editor.Insert("z 22\na 1\nm 3")
	doc.Editor.ClearHistory()
	rect := Rectangle{Anchor: 2, Caret: doc.Editor.RuneCount(), CaretVirtual: 1, TabWidth: 4}
	doc.Editor.MoveTo(rect.Caret, false)
	after, accepted, err := doc.SortRectangleLocale(rect, language.Und, true)
	if err != nil || !accepted || doc.Editor.String() != "a 1\nm 3\nz 22" || after.Anchor != 2 || after.Caret != doc.Editor.RuneCount() || after.CaretVirtual != 0 {
		t.Fatalf("native rectangle: %q %+v %v %v", doc.Editor.String(), after, accepted, err)
	}
}
