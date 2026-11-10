//go:build windows

package document

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/text/language"
)

const maxNLSLocaleName = 85

var (
	nlsKernel32      = windows.NewLazySystemDLL("kernel32.dll")
	nlsMapStringEx   = nlsKernel32.NewProc("LCMapStringEx")
	nlsResolveLocale = nlsKernel32.NewProc("ResolveLocaleName")
)

type windowsLocaleKeys struct {
	locale language.Tag
	native *nlsLocaleKeys
}

func newLocaleKeys(locale language.Tag) localeKeys {
	return &windowsLocaleKeys{locale: locale}
}

func (keys *windowsLocaleKeys) key(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	if keys.native == nil {
		name, err := resolveNLSLocale(keys.locale)
		if err != nil {
			return nil, err
		}
		keys.native = &nlsLocaleKeys{locale: name, mapString: mapNLSString}
	}
	return keys.native.key(value)
}

func resolveNLSLocale(locale language.Tag) ([]uint16, error) {
	if locale == language.Und {
		return nil, nil
	}
	if err := nlsResolveLocale.Find(); err != nil {
		return nil, fmt.Errorf("Windows NLS locale resolver: %w", err)
	}
	name, err := windows.UTF16FromString(locale.String())
	if err != nil {
		return nil, err
	}
	result := make([]uint16, maxNLSLocaleName)
	count, _, callError := nlsResolveLocale.Call(uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&result[0])), uintptr(len(result)))
	runtime.KeepAlive(name)
	runtime.KeepAlive(result)
	if count == 0 {
		return nil, fmt.Errorf("Windows NLS locale resolution failed: %v", callError)
	}
	if count > uintptr(len(result)) || result[count-1] != 0 {
		return nil, fmt.Errorf("Windows NLS locale resolution returned an invalid name")
	}
	return result[:count], nil
}

func mapNLSString(locale []uint16, flags uint32, source []uint16, destination []byte) (int, error) {
	if err := nlsMapStringEx.Find(); err != nil {
		return 0, err
	}
	var localePointer *uint16
	var destinationPointer *byte
	if len(locale) > 0 {
		localePointer = &locale[0]
	}
	if len(destination) > 0 {
		destinationPointer = &destination[0]
	}
	result, _, callError := nlsMapStringEx.Call(
		uintptr(unsafe.Pointer(localePointer)), uintptr(flags), uintptr(unsafe.Pointer(&source[0])), uintptr(len(source)),
		uintptr(unsafe.Pointer(destinationPointer)), uintptr(len(destination)), 0, 0, 0,
	)
	runtime.KeepAlive(locale)
	runtime.KeepAlive(source)
	runtime.KeepAlive(destination)
	if result == 0 {
		if callError == windows.ERROR_SUCCESS {
			return 0, nil
		}
		return 0, callError
	}
	return int(int32(result)), nil
}
