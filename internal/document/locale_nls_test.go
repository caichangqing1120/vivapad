package document

import (
	"bytes"
	"errors"
	"math"
	"slices"
	"testing"

	"golang.org/x/text/language"
)

func TestNLSKeysUseSourceFlagsExplicitUTF16AndOpaqueBytes(t *testing.T) {
	name := []uint16{'e', 'n', '-', 'U', 'S', 0}
	wantSource := []uint16{'A', 0, 0xd83d, 0xde00, 0x732b}
	wantKey := []byte{0x10, 0, 0x20, 0, 0x30}
	calls := 0
	keys := &nlsLocaleKeys{locale: name, mapString: func(locale []uint16, flags uint32, source []uint16, destination []byte) (int, error) {
		calls++
		if !slices.Equal(locale, name) || flags != 0x08000418 || !slices.Equal(source, wantSource) {
			t.Fatalf("NLS arguments: locale=%v flags=%08x source=%x", locale, flags, source)
		}
		if calls == 1 {
			if len(destination) != 0 {
				t.Fatal("sizing call supplied a destination")
			}
			return len(wantKey), nil
		}
		if calls != 2 || len(destination) != len(wantKey) {
			t.Fatalf("NLS fill count is not bytes: calls=%d size=%d", calls, len(destination))
		}
		copy(destination, wantKey)
		return len(wantKey), nil
	}}
	got, err := keys.key("A\x00\U0001f600猫")
	if err != nil || !bytes.Equal(got, wantKey) || calls != 2 {
		t.Fatalf("opaque key was changed: %x, %v (%d calls)", got, err, calls)
	}
}

func TestNLSKeysSkipEmptyAndPreserveActualFillLength(t *testing.T) {
	calls := 0
	keys := &nlsLocaleKeys{mapString: func(locale []uint16, _ uint32, _ []uint16, destination []byte) (int, error) {
		calls++
		if locale != nil {
			t.Fatal("default NLS locale must be null")
		}
		if len(destination) == 0 {
			return 5, nil
		}
		copy(destination, []byte{4, 0, 3})
		return 3, nil
	}}
	if key, err := keys.key(""); err != nil || key != nil || calls != 0 {
		t.Fatal("empty key invoked NLS")
	}
	if key, err := keys.key("value"); err != nil || !bytes.Equal(key, []byte{4, 0, 3}) || calls != 2 {
		t.Fatalf("fill result length was not honored: %x %v", key, err)
	}
}

func TestNLSKeysRefuseInvalidQueryAndFillResults(t *testing.T) {
	failure := errors.New("NLS failed")
	oversized := uint64(math.MaxInt32) + 1
	for _, test := range []struct {
		name       string
		query      int
		queryError error
		written    int
		fillError  error
	}{
		{"query error", 0, failure, 0, nil},
		{"zero query", 0, nil, 0, nil},
		{"negative query", -1, nil, 0, nil},
		{"oversized query", int(oversized), nil, 0, nil},
		{"fill error", 3, nil, 0, failure},
		{"zero fill", 3, nil, 0, nil},
		{"negative fill", 3, nil, -1, nil},
		{"oversized fill", 3, nil, 4, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			keys := &nlsLocaleKeys{mapString: func(_ []uint16, _ uint32, _ []uint16, destination []byte) (int, error) {
				calls++
				if len(destination) == 0 {
					return test.query, test.queryError
				}
				return test.written, test.fillError
			}}
			key, err := keys.key("value")
			if err == nil || key != nil || (test.queryError != nil || test.fillError != nil) && !errors.Is(err, failure) {
				t.Fatalf("invalid NLS result accepted: %x %v", key, err)
			}
			if test.query <= 0 || int64(test.query) > math.MaxInt32 || test.queryError != nil {
				if calls != 1 {
					t.Fatalf("invalid query reached a fill call: %d", calls)
				}
			}
		})
	}
}

func TestLocaleKeyBuffersRemainOwnedAcrossCalls(t *testing.T) {
	for _, keys := range []localeKeys{
		newLocaleKeys(language.English),
		&nlsLocaleKeys{mapString: func(_ []uint16, _ uint32, source []uint16, destination []byte) (int, error) {
			if len(destination) != 0 {
				destination[0] = byte(source[0])
			}
			return 1, nil
		}},
	} {
		first, err := keys.key("first")
		if err != nil {
			t.Fatal(err)
		}
		copyOfFirst := bytes.Clone(first)
		if _, err := keys.key("second"); err != nil || !bytes.Equal(first, copyOfFirst) {
			t.Fatal("later key overwrote an earlier key")
		}
	}
}
