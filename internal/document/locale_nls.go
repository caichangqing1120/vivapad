package document

import (
	"fmt"
	"math"
	"unicode/utf16"
)

const defaultNLSSortFlags uint32 = 0x00000400 | 0x08000000 | 0x00000010 | 0x00000008

type nlsMapCall func(locale []uint16, flags uint32, source []uint16, destination []byte) (int, error)

type nlsLocaleKeys struct {
	locale    []uint16
	mapString nlsMapCall
}

func (keys *nlsLocaleKeys) key(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	if keys.mapString == nil {
		return nil, fmt.Errorf("Windows NLS key mapper is unavailable")
	}
	value = value[:min(len(value), math.MaxInt32/2)]
	units := 0
	for _, character := range value {
		units += utf16.RuneLen(character)
	}
	source := make([]uint16, 0, units)
	for _, character := range value {
		source = utf16.AppendRune(source, character)
	}
	size, err := keys.mapString(keys.locale, defaultNLSSortFlags, source, nil)
	if err != nil {
		return nil, fmt.Errorf("Windows NLS key sizing: %w", err)
	}
	if size <= 0 || int64(size) > math.MaxInt32 {
		return nil, fmt.Errorf("Windows NLS key sizing returned %d bytes", size)
	}
	result := make([]byte, size)
	written, err := keys.mapString(keys.locale, defaultNLSSortFlags, source, result)
	if err != nil {
		return nil, fmt.Errorf("Windows NLS key fill: %w", err)
	}
	if written <= 0 || written > len(result) {
		return nil, fmt.Errorf("Windows NLS key fill returned %d of %d bytes", written, len(result))
	}
	return result[:written], nil
}
